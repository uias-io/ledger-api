-- 移除公共分类，收敛字段
ALTER TABLE ledger_category
    MODIFY COLUMN instance_id char(32)
    CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci
    NOT NULL;

-- 新增标签表外键
ALTER TABLE ledger_tagrel
    MODIFY COLUMN bill_id INT(11) NOT NULL;

ALTER TABLE ledger_tagrel
    ADD CONSTRAINT fk_tagrel_bill
        FOREIGN KEY (bill_id) REFERENCES ledger_transaction (kid)
            ON DELETE CASCADE ON UPDATE CASCADE;
