-- 删除旧表
DROP TABLE IF EXISTS ledger_trantag;
DROP TABLE IF EXISTS ledger_tag;

-- 新增标签表
CREATE TABLE IF NOT EXISTS `ledger_tagrel` (
  `kid` BIGINT NOT NULL AUTO_INCREMENT,
  `instance_id` char(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL,
  `bill_id` BIGINT NOT NULL,
  `name` VARCHAR(64) NOT NULL,
  `create_time` BIGINT NOT NULL,
  PRIMARY KEY (`kid`) USING BTREE,
  UNIQUE KEY `uk_instance_bill_name` (`instance_id`, `bill_id`, `name`) USING BTREE,
  INDEX `idx_instance_name_ct` (`instance_id`, `name`, `create_time`) USING BTREE,
  INDEX `idx_bill_id` (`bill_id`) USING BTREE,
  CONSTRAINT `ledger_tagrel_ibfk_1` FOREIGN KEY (`instance_id`) REFERENCES `ledger_instance` (`instance_id`) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE = InnoDB AUTO_INCREMENT = 1000000 CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci ROW_FORMAT = DYNAMIC;
