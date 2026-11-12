package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// OrmTransaction 收支明细表
type OrmTransaction struct {
	Kid        int64           `gorm:"column:kid;primaryKey"` // 主键ID
	InstanceId string          `gorm:"column:instance_id"`    // 实例ID
	CategoryId int64           `gorm:"column:category_id"`    // 分类ID
	Amount     decimal.Decimal `gorm:"column:amount"`         // 金额,0.0000
	OccTime    string          `gorm:"column:occ_time"`       // 账目发生时间(occurrence)
	OccAt      int64           `gorm:"column:occ_at"`         // 账目发生时间戳(occurrence)
	Remark     string          `gorm:"column:remark"`         // 详细备注
	UpdateTime int64           `gorm:"column:update_time"`    // 记录时间
	CreateTime int64           `gorm:"column:create_time"`    // 更新时间
}

func (OrmTransaction) TableName() string {
	return TableNameTransaction
}

// validateOccAt 校验发生时间戳，0 视为非法
func validateOccAt(occAt int64) error {
	if occAt == 0 {
		return fmt.Errorf("invalid occ_at: must not be zero")
	}
	return nil
}

var ErrTransactionNotFound = errors.New("transaction not found")

// ensureTransactionOwned 校验主单属于该实例，防跨租户操作子表。
func ensureTransactionOwned(tx *gorm.DB, instanceId string, tid int64) error {
	var cnt int64
	if err := tx.Model(&OrmTransaction{}).
		Where("instance_id = ? AND kid = ?", instanceId, tid).
		Count(&cnt).Error; err != nil {
		return fmt.Errorf("check transaction ownership: %w", err)
	}
	if cnt == 0 {
		return fmt.Errorf("%w: id=%d instance=%s", ErrTransactionNotFound, tid, instanceId)
	}
	return nil
}

// replaceDetails 替换明细,则先删后建。
func replaceDetails(tx *gorm.DB, tid int64, detailList []*OrmDetail) error {
	if detailList == nil {
		return nil
	}
	if err := tx.Where("transaction_id = ?", tid).
		Delete(&OrmDetail{}).Error; err != nil {
		return fmt.Errorf("delete detail: %w", err)
	}
	if len(detailList) == 0 {
		return nil
	}
	for i := range detailList {
		detailList[i].TransactionId = tid
	}
	if err := tx.Create(detailList).Error; err != nil {
		return fmt.Errorf("create detail: %w", err)
	}
	return nil
}

// replaceTags 替换标签,先删后建。
func replaceTags(tx *gorm.DB, instanceId string, billId int64, tags []string, createTime int64) error {
	if tags == nil {
		return nil
	}
	if err := tx.Where("instance_id = ? AND bill_id = ?", instanceId, billId).
		Delete(&OrmBillTagRel{}).Error; err != nil {
		return fmt.Errorf("delete bill tag: %w", err)
	}
	if len(tags) == 0 {
		return nil
	}
	ormTags := make([]*OrmBillTagRel, 0, len(tags))
	for _, v := range tags {
		ormTags = append(ormTags, &OrmBillTagRel{
			InstanceId: instanceId,
			BillId:     billId,
			Name:       v,
			CreateTime: createTime,
		})
	}
	if err := affairsCreateTag(tx, ormTags); err != nil {
		return fmt.Errorf("create bill tag: %w", err)
	}
	return nil
}

// normalizeTags 去重 + trim + 去空。
func normalizeTags(tags []string) []string {
	if tags == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	return out
}

// InstallTransaction 新增交易（主单 + 明细 + 标签）
func (d DB) InstallTransaction(transaction *OrmTransaction, detailList []*OrmDetail, tags []string) error {
	if err := validateOccAt(transaction.OccAt); err != nil {
		return err
	}

	if transaction.InstanceId != "" && transaction.InstanceId != d.instanceId {
		return fmt.Errorf("transaction instance_id %q mismatch with db instance %q",
			transaction.InstanceId, d.instanceId)
	}
	transaction.InstanceId = d.instanceId

	if transaction.CreateTime == 0 {
		transaction.CreateTime = time.Now().UnixMilli()
	}
	if transaction.UpdateTime == 0 {
		transaction.UpdateTime = transaction.CreateTime
	}

	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(transaction).Error; err != nil {
			return fmt.Errorf("create transaction: %w", err)
		}
		if err := replaceDetails(tx, transaction.Kid, detailList); err != nil {
			return err
		}
		if err := replaceTags(tx, d.instanceId, transaction.Kid, normalizeTags(tags), transaction.CreateTime); err != nil {
			return err
		}
		return nil
	})
}

// DeleteTransaction 删除账单（主单 + 明细 + 标签）
func (d DB) DeleteTransaction(tid int64) error {
	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := ensureTransactionOwned(tx, d.instanceId, tid); err != nil {
			return err
		}

		if err := tx.Where("instance_id = ? AND bill_id = ?", d.instanceId, tid).
			Delete(&OrmBillTagRel{}).Error; err != nil {
			return fmt.Errorf("delete bill tag: %w", err)
		}
		if err := tx.Where("transaction_id = ?", tid).
			Delete(&OrmDetail{}).Error; err != nil {
			return fmt.Errorf("delete detail: %w", err)
		}
		if err := tx.Where("instance_id = ? AND kid = ?", d.instanceId, tid).
			Delete(&OrmTransaction{}).Error; err != nil {
			return fmt.Errorf("delete transaction: %w", err)
		}
		return nil
	})
}

// UpdateTransaction 更新账单
func (d DB) UpdateTransaction(tid int64, transaction map[string]any, detailList []*OrmDetail, tags []string) error {
	fields := make(map[string]any, len(transaction)+1)
	for k, v := range transaction {
		fields[k] = v
	}
	delete(fields, "create_time")

	if v, ok := fields["occ_at"]; ok {
		occAt, ok := v.(int64)
		if !ok {
			return fmt.Errorf("invalid occ_at type: %T", v)
		}
		if err := validateOccAt(occAt); err != nil {
			return err
		}
	}

	tags = normalizeTags(tags)

	return d.db.Transaction(func(tx *gorm.DB) error {
		if err := ensureTransactionOwned(tx, d.instanceId, tid); err != nil {
			return err
		}

		if len(fields) > 0 {
			fields["update_time"] = time.Now().UnixMilli()
			if err := tx.Model(&OrmTransaction{}).
				Where("instance_id = ? AND kid = ?", d.instanceId, tid).
				Updates(fields).Error; err != nil {
				return fmt.Errorf("update transaction: %w", err)
			}
		}

		// 明细
		if err := replaceDetails(tx, tid, detailList); err != nil {
			return err
		}

		// 标签
		if err := replaceTags(tx, d.instanceId, tid, tags, time.Now().UnixMilli()); err != nil {
			return err
		}

		return nil
	})
}

// FindTransactionForUser 按用户实例和id查询
func FindTransactionForUser(instanceId string, transactionId int64) ([]*OrmTransaction, error) {
	var detail []*OrmTransaction
	r := db.Model(&OrmTransaction{}).
		Where("instance_id = ? AND kid = ?", instanceId, transactionId).Find(&detail)
	return detail, r.Error
}

// GetBillById 查询账单
func GetBillById(instanceId string, billId int64) ([]*OrmTransaction, error) {
	var detail []*OrmTransaction
	err := db.Model(&OrmTransaction{}).
		Where("instance_id = ?", instanceId).
		Where("kid = ?", billId).
		Find(&detail).Error

	return detail, err
}

// TransactionFilter 账单查询条件，nil 表示该维度不过滤
type TransactionFilter struct {
	CategoryId *int64
	From       *int64           // 含
	To         *int64           // 不含
	MinAmount  *decimal.Decimal // 含
	MaxAmount  *decimal.Decimal // 含
	Keyword    *string
	Direction  *int8
	Limit      int
	Offset     int
	SortColumn string // "occ_at" | "amount" | "create_time"
	SortOrder  string // "ASC" | "DESC"
}

// FindTransactionByFilter returns a paginated list of transactions matching the search criteria.
func FindTransactionByFilter(instanceId string, f TransactionFilter, count *int64) ([]*OrmTransaction, error) {
	query := db.Model(&OrmTransaction{}).
		Where("instance_id = ?", instanceId)

	if f.CategoryId != nil {
		query = query.Where("category_id = ?", *f.CategoryId)
	}
	if f.From != nil {
		query = query.Where("occ_at >= ?", *f.From) // inclusive
	}
	if f.To != nil {
		query = query.Where("occ_at < ?", *f.To) // exclusive
	}
	if f.Keyword != nil && *f.Keyword != "" {
		kw := escapeLike(*f.Keyword)
		query = query.Where("remark LIKE ?", "%"+kw+"%")
	}

	// Direction: filter by the sign of amount, no join with the category table needed.
	if f.Direction != nil {
		switch *f.Direction {
		case 1: // income
			query = query.Where("amount > 0")
		case 2: // expense
			query = query.Where("amount < 0")
		}
	}

	// Amount range: expand into positive and negative segments, so 10 matches both +10 and -10.
	// The positive segment is constrained to amount > 0 and the negative segment to amount < 0,
	// preventing the two segments from extending into each other and the union collapsing to a tautology
	// when only one bound is given.
	// Direction 1 builds only the positive segment, Direction 2 only the negative segment;
	// nil or an invalid value (not 1/2) means no direction filter, so both segments are built.
	//   min=10,max=10  → (amount > 0 AND amount >= 10 AND amount <= 10) OR (amount < 0 AND amount <= -10 AND amount >= -10)
	//   min=10,max=100 → (amount > 0 AND amount >= 10 AND amount <= 100) OR (amount < 0 AND amount <= -10 AND amount >= -100)
	//   only min=10    → (amount > 0 AND amount >= 10) OR (amount < 0 AND amount <= -10)
	//   only max=100   → (amount > 0 AND amount <= 100) OR (amount < 0 AND amount >= -100)
	if f.MinAmount != nil || f.MaxAmount != nil {
		buildPos := f.Direction == nil || *f.Direction != 2
		buildNeg := f.Direction == nil || *f.Direction != 1

		var conds []*gorm.DB
		if buildPos {
			posCond := db.Where("amount > 0")
			if f.MinAmount != nil {
				posCond = posCond.Where("amount >= ?", *f.MinAmount)
			}
			if f.MaxAmount != nil {
				posCond = posCond.Where("amount <= ?", *f.MaxAmount)
			}
			conds = append(conds, posCond)
		}
		if buildNeg {
			negCond := db.Where("amount < 0")
			if f.MinAmount != nil {
				negCond = negCond.Where("amount <= ?", f.MinAmount.Neg())
			}
			if f.MaxAmount != nil {
				negCond = negCond.Where("amount >= ?", f.MaxAmount.Neg())
			}
			conds = append(conds, negCond)
		}

		if len(conds) == 1 {
			query = query.Where(conds[0])
		} else if len(conds) == 2 {
			query = query.Where(db.Where(conds[0]).Or(conds[1]))
		}
	}

	// Count the total before paginating.
	if err := query.Count(count).Error; err != nil {
		return nil, err
	}

	// Sorting: both the column and the order come from a whitelist; kid is used as a stable tie-breaker.
	orderClause := fmt.Sprintf("%s %s, kid %s", f.SortColumn, f.SortOrder, f.SortOrder)

	var result []*OrmTransaction
	err := query.Order(orderClause).Limit(f.Limit).Offset(f.Offset).Find(&result).Error
	return result, err
}

// escapeLike escapes LIKE wildcards.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
