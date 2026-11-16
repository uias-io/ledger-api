package models

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// OrmCategory 分类表
type OrmCategory struct {
	Kid        int64  `gorm:"column:kid;primaryKey"` // 主键ID
	InstanceId string `gorm:"column:instance_id"`    // 实例ID
	Name       string `gorm:"column:name"`           // 分类名称
	Direction  int8   `gorm:"column:direction"`      // 1=收入，2=支出
	Layer      int8   `gorm:"column:layer"`          // 1=大类，2=子类
	Sort       int    `gorm:"column:sort"`           // 同层同方向排序值
	CreateTime int64  `gorm:"column:create_time"`    // 创建时间
}

func (OrmCategory) TableName() string {
	return TableNameCategory
}

// OrmCaterela 分类关联表
type OrmCaterela struct {
	Kid        int64 `gorm:"column:kid;primaryKey"` // 主键ID
	ParentId   int64 `gorm:"column:parent_id"`      // 父ID
	ChildId    int64 `gorm:"column:child_id"`       // 子项ID
	CreateTime int64 `gorm:"column:create_time"`    // 创建时间
}

func (OrmCaterela) TableName() string {
	return TableNameCaterela
}

// CategoryWithRelResult 自定义结果结构体：接收关联查询后的字段（用rel.parent_id作为pid）
type CategoryWithRelResult struct {
	Kid        int64  `gorm:"column:kid" json:"kid"`                 // 对应ledger_category.kid
	InstanceId string `gorm:"column:instance_id" json:"instance_id"` // 对应ledger_category.instance_id
	Name       string `gorm:"column:name" json:"name"`               // 对应ledger_category.name
	Direction  int8   `gorm:"column:direction" json:"direction"`     // 对应ledger_category.direction
	Pid        int64  `gorm:"column:pid" json:"pid"`                 // 对应ledger_caterela.parent_id（别名pid）
	Sort       int    `gorm:"column:sort" json:"sort"`               // 对应ledger_category.sort
	CreateTime int64  `gorm:"column:create_time" json:"create_time"` // 对应ledger_category.create_time
}

// FindCategoryByType 查询收入或支出的分类
func FindCategoryByType(t string) ([]*OrmCategory, error) {
	var category []*OrmCategory
	err := db.Model(&OrmCategory{}).Where("type = ?", t).Find(&category).Error
	return category, err
}

// FindCategoryById 按kid查询分类
func FindCategoryById(instanceId string, kid int64) (*OrmCategory, error) {
	var category *OrmCategory
	r := db.Model(&OrmCategory{}).
		Where("instance_id = ?", instanceId).
		Where("kid = ?", kid).
		First(&category)
	return category, r.Error
}

// FindCategoryByUser 按类型查询用户的所有分类
func FindCategoryByUser(instanceId string, Direction int8) ([]*OrmCategory, error) {
	var category []*OrmCategory
	r := db.Model(&OrmCategory{}).
		Where("instance_id = ?", instanceId).
		Where("direction = ?", Direction).
		Find(&category)
	return category, r.Error
}

// GetCategoryWithRel 关联查询分类
func GetCategoryWithRel(instanceId string, direction int8) ([]*CategoryWithRelResult, error) {
	var results []*CategoryWithRelResult

	err := db.Table(TableNameCategory+" AS c").
		Select(`c.kid, c.instance_id, c.name, c.direction,
		        IFNULL(rel.parent_id, 0) AS pid, c.sort, c.create_time`).
		Joins("LEFT JOIN "+TableNameCaterela+" AS rel ON c.kid = rel.child_id").
		Where("c.instance_id = ?", instanceId).
		Where("c.direction = ?", direction).
		Order("c.layer, pid, c.sort, c.kid").
		Find(&results).Error

	if err != nil {
		return nil, err
	}

	return results, nil
}

// CreateCategory 创建分类（自动取同层同方向 max(sort)+1）
func CreateCategory(category *OrmCategory, rela *OrmCaterela) (int64, error) {
	var kid int64
	err := db.Transaction(func(tx *gorm.DB) error {
		var maxSort int
		if rela == nil {
			if err := tx.Model(&OrmCategory{}).
				Where("instance_id = ? AND layer = 1 AND direction = ?",
					category.InstanceId, category.Direction).
				Select("IFNULL(MAX(sort),0)").
				Scan(&maxSort).Error; err != nil {
				return fmt.Errorf("query max sort for parent: %w", err)
			}
		} else {
			if err := tx.Table(TableNameCategory+" AS c").
				Joins("JOIN "+TableNameCaterela+" AS rel ON rel.child_id = c.kid").
				Where("rel.parent_id = ?", rela.ParentId).
				Select("IFNULL(MAX(c.sort),0)").
				Scan(&maxSort).Error; err != nil {
				return fmt.Errorf("query max sort for child: %w", err)
			}
		}
		category.Sort = maxSort + 1

		if err := tx.Create(category).Error; err != nil {
			return err
		}
		kid = category.Kid
		if rela != nil {
			rela.ChildId = kid
			if err := tx.Create(rela).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return kid, err
}

// DeleteCategory 删除分类
func DeleteCategory(instanceId string, ids []int64) (int64, error) {
	r := db.Model(&OrmCategory{}).
		Where("instance_id = ?", instanceId).
		Where("kid IN (?)", ids).Delete(&OrmCategory{})
	return r.RowsAffected, r.Error
}

// DeleteCategoryWithRel 删除分类及其关联行（事务）
func DeleteCategoryWithRel(instanceId string, kid int64) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("child_id = ? OR parent_id = ?", kid, kid).
			Delete(&OrmCaterela{}).Error; err != nil {
			return fmt.Errorf("delete caterela: %w", err)
		}
		if err := tx.Where("instance_id = ? AND kid = ?", instanceId, kid).
			Delete(&OrmCategory{}).Error; err != nil {
			return fmt.Errorf("delete category: %w", err)
		}
		return nil
	})
}

// UpdateCategory 更新分类
func UpdateCategory(title string, pid uint32) error {
	r := db.Model(&OrmCategory{}).Updates(map[string]interface{}{
		"title":       title,
		"pid":         pid,
		"update_time": time.Now().UnixMilli(),
	})
	return r.Error
}

// UpdateCategoryFields 按实例更新分类字段
func UpdateCategoryFields(instanceId string, kid int64, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	r := db.Model(&OrmCategory{}).
		Where("instance_id = ? AND kid = ?", instanceId, kid).
		Updates(fields)
	return r.Error
}

// SortCategoryParent 重排某实例某方向的父类顺序
func SortCategoryParent(instanceId string, direction int8, ids []int64) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			if err := tx.Model(&OrmCategory{}).
				Where("instance_id = ? AND kid = ? AND layer = 1 AND direction = ?",
					instanceId, id, direction).
				Update("sort", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SortCategoryChildren 重排某父类下的子类顺序
func SortCategoryChildren(instanceId string, parentId int64, ids []int64) error {
	return db.Transaction(func(tx *gorm.DB) error {
		subQuery := tx.Model(&OrmCaterela{}).
			Select("child_id").
			Where("parent_id = ?", parentId)

		for i, id := range ids {
			if err := tx.Model(&OrmCategory{}).
				Where("instance_id = ? AND kid = ? AND layer = 2", instanceId, id).
				Where("kid IN (?)", subQuery).
				Update("sort", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// FindParentIdsByDirection 查某实例某方向的全部父类ID
func FindParentIdsByDirection(instanceId string, direction int8) ([]int64, error) {
	var ids []int64
	err := db.Model(&OrmCategory{}).
		Where("instance_id = ? AND layer = 1 AND direction = ?", instanceId, direction).
		Pluck("kid", &ids).Error
	return ids, err
}

// FindChildrenIdsByParent 查某父类下全部子类ID
func FindChildrenIdsByParent(parentId int64) ([]int64, error) {
	var ids []int64
	err := db.Model(&OrmCaterela{}).
		Where("parent_id = ?", parentId).
		Pluck("child_id", &ids).Error
	return ids, err
}

// ErrCategorySortMismatch 排序归属不匹配
var ErrCategorySortMismatch = errors.New("category sort scope mismatch")
