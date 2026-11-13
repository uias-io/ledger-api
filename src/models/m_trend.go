package models

import "github.com/shopspring/decimal"

// TrendTx 趋势统计用的明细
type TrendTx struct {
	OccAt  int64           `gorm:"column:occ_at"`
	Amount decimal.Decimal `gorm:"column:amount"`
}

// TrendTxs 只按时间过滤返回明细，不做分组
func (d *DB) TrendTxs(from, to int64) ([]*TrendTx, error) {
	var list []*TrendTx
	err := d.WithInstance().
		Table(TableNameTransaction).
		Where("occ_at >= ? AND occ_at < ?", from, to).
		Select("occ_at, amount").
		Order("occ_at").
		Find(&list).Error
	return list, err
}
