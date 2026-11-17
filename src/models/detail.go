package models

import (
	"github.com/shopspring/decimal"
)

// OrmDetail 明细详情
type OrmDetail struct {
	Kid           int64           `gorm:"column:kid;primaryKey"` // 主键ID
	InstanceId    string          `gorm:"column:instance_id"`    // 实例ID
	TransactionId int64           `gorm:"column:transaction_id"` // 交易ID
	Name          string          `gorm:"column:name"`           // 物品名称（如“牛奶”）
	Quantity      decimal.Decimal `gorm:"column:quantity"`       // 数量（支持小数）
	Unit          string          `json:"unit"`                  // 单位
	Price         decimal.Decimal `gorm:"column:price"`          // 单价
	Total         decimal.Decimal `gorm:"column:total"`          // 该项总价
	Remark        string          `gorm:"column:remark"`         // 详细备注
}

func (OrmDetail) TableName() string {
	return TableNameDetail
}

// GetDetail 查询明细
func GetDetail(instanceId string, billId int64) ([]*OrmDetail, error) {
	var detail []*OrmDetail
	err := db.Model(&OrmDetail{}).
		Where("instance_id = ?", instanceId).
		Where("transaction_id = ?", billId).
		Find(&detail).Error

	return detail, err
}

// DetailFilter 明细查询条件，nil 表示该维度不过滤
type DetailFilter struct {
	TransactionId *int64  // 按账单过滤
	Keyword       *string // 名称/备注模糊搜索
	Limit         int
	Offset        int
}

type DetailWithTime struct {
	OrmDetail
	OccAt int64 `gorm:"column:occ_at"`
}

// FindDetailByFilter 按实例查询明细
func FindDetailByFilter(instanceId string, f DetailFilter, count *int64) ([]*DetailWithTime, error) {
	query := db.Table(TableNameDetail+" AS d").
		Joins("JOIN "+TableNameTransaction+" AS t ON t.kid = d.transaction_id").
		Where("d.instance_id = ?", instanceId)

	if f.TransactionId != nil {
		query = query.Where("d.transaction_id = ?", *f.TransactionId)
	}
	if f.Keyword != nil && *f.Keyword != "" {
		kw := escapeLike(*f.Keyword)
		query = query.Where("(d.name LIKE ? OR d.remark LIKE ?)", "%"+kw+"%", "%"+kw+"%")
	}

	if err := query.Count(count).Error; err != nil {
		return nil, err
	}

	var result []*DetailWithTime
	err := query.
		Select("d.kid, d.instance_id, d.transaction_id, d.name, d.quantity, d.unit, d.price, d.total, d.remark, t.occ_at").
		Order("t.occ_at DESC, d.kid DESC").
		Limit(f.Limit).Offset(f.Offset).
		Scan(&result).Error
	return result, err
}
