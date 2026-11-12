package models

import "gorm.io/gorm"

type OrmBillTagRel struct {
	Kid        int64  `gorm:"column:kid;primaryKey"`
	InstanceId string `gorm:"column:instance_id"`
	BillId     int64  `gorm:"column:bill_id"`
	Name       string `gorm:"column:name"`
	CreateTime int64  `gorm:"column:create_time"`
}

func (OrmBillTagRel) TableName() string {
	return TableNameTagRel
}

type TagStat struct {
	Name        string `json:"name"`
	UseCount    int64  `json:"use_count"`
	LastUseTime int64  `json:"last_use_time"`
}

func affairsCreateTag(tx *gorm.DB, tags []*OrmBillTagRel) error {
	if len(tags) == 0 {
		return nil
	}
	return tx.Create(tags).Error
}

// ListHotTags 获取高频推荐标签
func ListHotTags(instanceId string, limit int) ([]TagStat, error) {
	var list []TagStat
	err := db.Model(&OrmBillTagRel{}).
		Select("name, COUNT(*) AS use_count, MAX(create_time) AS last_use_time").
		Where("instance_id = ?", instanceId).
		Group("name").
		Order("use_count DESC, last_use_time DESC").
		Limit(limit).
		Scan(&list).Error
	return list, err
}

// SearchTags 模糊搜索历史标签
func SearchTags(instanceId string, keyword string, limit int) ([]TagStat, error) {
	var list []TagStat
	err := db.Model(&OrmBillTagRel{}).
		Select("name, MAX(create_time) AS last_use_time").
		Where("instance_id = ? AND name LIKE ?", instanceId, "%"+keyword+"%").
		Group("name").
		Order("last_use_time DESC").
		Limit(limit).
		Scan(&list).Error
	return list, err
}

// GetBillTagNames 查询某条账单全部标签
func GetBillTagNames(instanceId string, billId int64) ([]string, error) {
	var names []string
	err := db.Model(&OrmBillTagRel{}).
		Select("name").
		Where("instance_id = ? AND bill_id = ?", instanceId, billId).
		Scan(&names).Error
	return names, err
}
