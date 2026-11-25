package config

import (
	"context"
	"fmt"
	"ledger/src/pkg/crypto"
	"ledger/src/zlog"
	"os"
)

// encryption 命令行加密
func encryption(plain string) {
	fmt.Println(crypto.Encryption(plain))
	os.Exit(0)
}

// decryCfgCipher 解密配置文件密文
func decryCfgCipher(cipherText *string) {
	if cipherText != nil && *cipherText == "" {
		return
	}
	plain, err := crypto.Decryption(*cipherText)
	if err != nil {
		zlog.Errorf(context.Background(), "Decryption password failed. %v", cipherText)
		os.Exit(100)
	}
	*cipherText = plain
}
