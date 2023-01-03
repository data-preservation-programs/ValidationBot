package retrieval

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBitswapBuilderImpl_Build(t *testing.T) {
	assert := assert.New(t)
	retriever, _, closer := getRetriever(t)
	defer closer()
	assert.NotNil(retriever)
}
