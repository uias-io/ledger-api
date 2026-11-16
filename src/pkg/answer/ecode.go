package answer

const (
	EcodeOK    = "Ledger.0000" // 结果正常
	EcodeError = "Ledger.0101" // 结果错误（兜底/内部错误）

	// ---------------- 01xx 请求参数类 ----------------
	EcodeQueryError                = "Ledger.0102" // 查询参数错误
	EcodeInvalidRequestParamsError = "Ledger.0103" // 请求参数校验失败
	EcodeInvalidRequestError       = "Ledger.0104" // 请求体错误（JSON 解析失败等）
	EcodeInvalidPathParam          = "Ledger.0105" // 路径参数非法（如 id 非数字）
	EcodeNoFieldsToUpdate          = "Ledger.0106" // 没有可更新的字段
	EcodeInvalidPagingParam        = "Ledger.0107" // 分页参数非法
	EcodeInvalidTimeRange          = "Ledger.0108" // 时间范围/周期参数非法
	EcodeInvalidSortRequest        = "Ledger.0109" // 排序请求参数非法（categoryIds 为空/含非法项/重复）

	// ---------------- 02xx 资源不存在 ----------------
	EcodeCategoryNotFound     = "Ledger.0201" // 分类不存在
	EcodeTransactionNotFound  = "Ledger.0202" // 账单不存在
	EcodeTagNotFound          = "Ledger.0203" // 标签不存在
	EcodeSortCategoryNotFound = "Ledger.0204" // 排序目标分类不存在

	// ---------------- 03xx 资源冲突 / 状态（避开上游 0301） ----------------
	EcodeResourceAlreadyExists = "Ledger.0311" // 资源已存在（唯一键冲突）
	EcodeCategoryHasChildren   = "Ledger.0312" // 分类存在子分类，无法删除
	EcodeCategoryInUse         = "Ledger.0313" // 分类被账单引用，无法删除
	EcodeSortCategoryMismatch  = "Ledger.0314" // 排序分类归属不匹配（跨层/跨父类/跨方向/跨实例）

	// ---------------- 04xx 权限 / 归属 ----------------
	EcodePermissionDenied        = "Ledger.0401" // 无权操作该资源（跨实例）
	EcodeCategorySystemProtected = "Ledger.0402" // 系统分类不允许修改/删除

	// ---------------- 05xx 业务规则 ----------------
	EcodeCategoryInvalidDirection   = "Ledger.0501" // 分类方向非法（仅 1/2）
	EcodeTransactionInvalidOccTime  = "Ledger.0502" // 发生时间非法
	EcodeTransactionAmountDirection = "Ledger.0503" // 金额方向与分类方向不一致
	EcodeTransactionDetailExceed    = "Ledger.0504" // 明细总价超过账单金额
	EcodeTransactionAmountZero      = "Ledger.0505" // 金额不能为 0
	EcodeSortCategoryIncomplete     = "Ledger.0506" // 排序分类列表不完整（缺失同层分类）

	// ---------------- 鉴权类（保持原值） ----------------
	EcodeInvalidTokenError   = "Ledger.0175" // 无效/错误的 token
	EcodeNoActionError       = "Ledger.0177" // action 不存在
	EcodePolicyNotAuthorized = "Ledger.0178" // 策略未授权此操作

	EcodeDeleteResourceConflict = "Ledger.0198" // 409 删除资源时冲突

	EcodeUpstreamResponseError = "Ledger.0301" // 上游返回处理错误
)
