package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"mochi-api/common"
	"mochi-api/model"
)

func TestListRelayModelsKeepsLiteralAliasModelVisible(t *testing.T) {
	common.DataDir = t.TempDir()
	require.NoError(t, model.InitDB())
	t.Cleanup(func() {
		if sqlDB, err := model.DB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	require.NoError(t, model.CreateChannel(&model.Channel{
		Name: "literal alias model", Type: model.ChannelTypeOpenAI,
		BaseURL: "https://example.com", Models: "gpt-5.6-sol",
		Status: model.StatusEnabled,
	}))
	require.NoError(t, model.CreateModelMapping(&model.ModelMapping{
		Alias: "gpt-5.6-sol", UpstreamName: "compatible-upstream-model",
	}))

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	ListRelayModels(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "gpt-5.6-sol", gjson.Get(recorder.Body.String(), "data.0.id").String())
	require.Equal(t, int64(1), gjson.Get(recorder.Body.String(), "data.#").Int())
}
