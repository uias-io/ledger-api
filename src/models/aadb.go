package models

import (
	"errors"
	"ledger/src/pkg/common"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var db *gorm.DB

const (
	TableNameInstance    = "ledger_instance"    // 实例表
	TableNameCategory    = "ledger_category"    // 分类表
	TableNameCaterela    = "ledger_caterela"    // 分类关联表
	TableNameTransaction = "ledger_transaction" // 账单表
	TableNameDetail      = "ledger_detail"      // 账单详情表
	TableNameTagRel      = "ledger_tagrel"      // 标签表
)

// SetConnectPool 设置models层db连接
func SetConnectPool(_db *gorm.DB) {
	db = _db
}

type DB struct {
	db          *gorm.DB      // DB 实例
	Klog        *logrus.Entry // 日志对象
	instanceId  string        // 实例ID
	CurrentTime int64         // 当前时间
}

// NewDBModel 创建DB实例
func NewDBModel(instanceId string) (*DB, error) {
	if instanceId == "" {
		return nil, errors.New("instanceId can not be empty")
	}
	_t := &DB{
		instanceId:  instanceId,
		db:          db,
		CurrentTime: common.CreateTimestamp(), // 设置默认当前时间
	}
	return _t, nil
}

// SetKlog 设置日志对象(可选)
func (d *DB) SetKlog(klog *logrus.Entry) *DB {
	d.Klog = klog
	return d
}

// SetCurrentTime 设置当前时间(可选)
func (d *DB) SetCurrentTime(currentTime int64) *DB {
	d.CurrentTime = currentTime
	return d
}

// WithInstance 带按实例ID查询的DB对象
func (d *DB) WithInstance() *gorm.DB {
	return d.db.Where("instance_id = ?", d.instanceId)
}
