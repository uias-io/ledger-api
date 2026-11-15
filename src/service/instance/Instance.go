package instance

import (
	"errors"
	"fmt"
	"ledger/src/global"
	"ledger/src/models"
	"ledger/src/pkg/common"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

const (
	InstanceValidity = 3600 * 2               // 实例ID缓存有效期(秒)
	retryMax         = 3                      // 最大重试次数
	retryDelay       = 100 * time.Millisecond // 重试延迟
)

const mysqlErrDuplicateEntry = 1062 // MySQL: Duplicate entry for key

var (
	// ErrInvalidUserId 用户ID非法（空或格式错误），上层可按参数错误处理
	ErrInvalidUserId = errors.New("invalid user id")

	// 用户级别的锁映射，避免并发创建
	userLocks sync.Map
)

// GetInstanceId 获取用户的实例ID，确保每个用户只有一个实例
func GetInstanceId(userId string) (string, error) {
	// 1. 前置校验：用户ID非空且格式合法
	if userId == "" {
		return "", fmt.Errorf("%w: user id is empty", ErrInvalidUserId)
	}
	if !common.IsValid32UUID(userId) {
		return "", fmt.Errorf("%w: must be a 32-character UUID", ErrInvalidUserId)
	}

	// 2. 优先查询缓存：命中则直接返回（减少DB访问）
	if instanceId, ok := global.GCache.Get(userId); ok {
		return instanceId, nil
	}

	// 3. 缓存未命中：查询数据库
	instance, err := models.GetInstance(userId)
	switch {
	// 3.1 数据库未找到记录：尝试创建新实例
	case errors.Is(err, gorm.ErrRecordNotFound):
		instanceId, err := createInstanceSafely(userId)
		if err != nil {
			return "", fmt.Errorf("failed to create instance for user %s: %w", userId, err)
		}
		return instanceId, nil

	// 3.2 数据库查询错误：直接返回错误
	case err != nil:
		return "", fmt.Errorf("failed to query instance from DB: %w", err)

	// 3.3 数据库查询成功：更新缓存并返回
	default:
		instanceId := instance.InstanceId
		global.GCache.Set(userId, instanceId, InstanceValidity)
		return instanceId, nil
	}
}

// createInstanceSafely 安全地为用户创建实例，避免重复创建
// 使用 per-user 互斥锁串行化创建流程；持锁期间做双重检查。
// 注意：锁用 defer 释放（不删除 map 条目），避免「删除锁导致其他 goroutine 拿到已释放锁」的竞态。
func createInstanceSafely(userId string) (string, error) {
	lockIface, _ := userLocks.LoadOrStore(userId, &sync.Mutex{})
	lock := lockIface.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	// 双重检查：持锁后再查一次，可能已被其他请求创建
	existingInstance, err := models.GetInstance(userId)
	if err == nil {
		global.GCache.Set(userId, existingInstance.InstanceId, InstanceValidity)
		return existingInstance.InstanceId, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", fmt.Errorf("failed to check instance existence: %w", err)
	}

	// 准备创建新实例
	newInstance := models.OrmInstance{
		InstanceId: common.CreateUuid(),
		UserId:     userId,
		CreateTime: common.CreateTimestamp(),
	}

	var lastErr error
	for i := 0; i < retryMax; i++ {
		if err := models.InstallInstance(newInstance); err != nil {
			// MySQL 唯一键冲突（user_id 已存在）：直接查一次返回已存在实例
			if isDuplicateKeyErr(err) {
				inst, qerr := models.GetInstance(userId)
				if qerr == nil {
					global.GCache.Set(userId, inst.InstanceId, InstanceValidity)
					return inst.InstanceId, nil
				}
			}
			lastErr = fmt.Errorf("db insert failed (userID: %s, instanceID: %s): %w",
				userId, newInstance.InstanceId, err)
			time.Sleep(retryDelay)
			continue
		}
		global.GCache.Set(userId, newInstance.InstanceId, InstanceValidity)
		return newInstance.InstanceId, nil
	}

	return "", lastErr
}

// isDuplicateKeyErr 判断是否为 MySQL 唯一键冲突（错误号 1062）
func isDuplicateKeyErr(err error) bool {
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == mysqlErrDuplicateEntry
	}
	return false
}
