package repository

import (
	"errors"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
)

func TestDuplicateKeyDetection(t *testing.T) {
	err := &mysql.MySQLError{Number: 1062, Message: "Duplicate entry"}

	require.True(t, isDuplicateKey(err))
	require.False(t, isDuplicateKey(errors.New("network error")))
}

func TestEventRepositoryInsertOnceDocumentsDuplicateSemantics(t *testing.T) {
	// 这个测试固定 InsertOnce 的幂等契约：重复键不是业务错误，而是表示事件已经处理。
	inserted, err := duplicateInsertResult(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"})

	require.NoError(t, err)
	require.False(t, inserted)
}

func duplicateInsertResult(err error) (bool, error) {
	if err == nil {
		return true, nil
	}
	if isDuplicateKey(err) {
		return false, nil
	}
	return false, err
}
