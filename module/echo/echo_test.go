package echo_test

import (
	"context"
	"testing"

	"validation-bot/auditor/echo"

	"github.com/stretchr/testify/assert"
)

func TestValidator_Validate(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	val := echo.Validator{}
	task := []byte("{\"type\":\"echo\",\"target\":\"target\",\"input\":\"hello\"}")
	result, err := val.Validate(context.TODO(), task)
	assert.Nil(err)
	resultString := string(result)
	assert.Equal("{\"echo\":\"hello\"}", resultString)
}

func TestValidator_InvalidInput(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	val := echo.Validator{}
	task := []byte("invalid")
	result, err := val.Validate(context.TODO(), task)
	assert.Nil(result)
	assert.NotNil(err)
}
