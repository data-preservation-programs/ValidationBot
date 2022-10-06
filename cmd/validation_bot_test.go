package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"validation-bot/task"
	"validation-bot/test"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

/*
 * Prerequisites:
 * 1. Install postgresql
 * 2. Create user same as the current user
 * 3. Create database test
 */
const (
	testPeerId = "12D3KooWG8tR9PHjjXcMknbNPVWT75BuXXA2RaYx3fMwwg2oPZXd"
	testUuid   = "d54ec3a8-4045-11ed-b878-0242ac120002"
)

func TestSetDefault(t *testing.T) {
	assert := assert.New(t)
	defer viper.Reset()
	setDefault("")
	assert.True(viper.GetBool("module.echo.enabled"))
	assert.False(viper.GetBool("dispatcher.enabled"))
}

func TestSetDefault_WithConfigFile(t *testing.T) {
	assert := assert.New(t)
	defer viper.Reset()
	setDefault("../config/config.toml")
	assert.True(viper.GetBool("module.echo.enabled"))
	assert.True(viper.GetBool("dispatcher.enabled"))
}

func TestDeleteTaskHandler(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	defer viper.Reset()
	mockTaskRemover := new(MockTaskRemover)
	mockTaskRemover.On("Remove", mock.Anything, mock.Anything).Return(nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/task/"+testUuid, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath(deleteRoute)
	c.SetParamNames("id")
	c.SetParamValues(testUuid)
	err := deleteTaskHandler(c, mockTaskRemover)
	assert.Nil(err)
	assert.Equal(http.StatusOK, rec.Code)
	mockTaskRemover.AssertCalled(t, "Remove", mock.Anything, uuid.MustParse(testUuid))
}

func TestPostTaskHandler(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	defer viper.Reset()
	taskDef := task.Definition{
		Definition: "definition",
	}
	mockTaskCreator := new(MockTaskCreator)
	mockTaskCreator.On("Create", mock.Anything, mock.Anything).Return(nil)
	e := echo.New()
	buf := bytes.NewBuffer([]byte("{\"definition\":\"definition\"}"))
	req := httptest.NewRequest(http.MethodPost, createRoute, buf)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath(createRoute)
	err := postTaskHandler(c, mockTaskCreator)
	assert.Nil(err)
	assert.Equal(http.StatusOK, rec.Code)
	mockTaskCreator.AssertCalled(t, "Create", mock.Anything, &taskDef)
}

func TestListTaskHandler(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)
	defer viper.Reset()
	mockTaskCreator := new(MockTaskCreator)
	taskDefs := []task.Definition{
		{
			Definition: "definition",
		},
	}
	mockTaskCreator.On("List", mock.Anything).Return(taskDefs, nil)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, listRoute, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath(listRoute)
	err := listTasksHandler(c, mockTaskCreator)
	assert.Nil(err)
	assert.Equal(http.StatusOK, rec.Code)
	mockTaskCreator.AssertCalled(t, "List", mock.Anything)
	assert.Equal("[{\"id\":\"00000000-0000-0000-0000-000000000000\",\"target\":\"\",\"type\":\"\",\"interval_seconds\":0,\"definition\":\"definition\",\"DispatchedTimes\":0,\"CreatedAt\":\"0001-01-01T00:00:00Z\",\"UpdatedAt\":\"0001-01-01T00:00:00Z\"}]\n", rec.Body.String())
}

func TestNewObserver(t *testing.T) {
	assert := assert.New(t)
	defer viper.Reset()
	viper.Set("observer.database_connection_string", test.PostgresConnectionString)
	viper.Set("observer.trusted_peers", []string{testPeerId})
	viper.Set("module.echo.enabled", true)
	obs, err := newObserver()
	assert.NotNil(obs)
	assert.Nil(err)
}

func TestNewAuditor(t *testing.T) {
	assert := assert.New(t)
	defer viper.Reset()
	private, _, _ := test.GeneratePeerID(t)
	privateKey := test.MarshalPrivateKey(t, private)
	viper.Set("auditor.private_key", privateKey)
	viper.Set("auditor.listen_addr", "/ip4/0.0.0.0/tcp/0")
	aud, err := newAuditor(context.TODO())
	assert.NotNil(aud)
	assert.Nil(err)
}

func TestNewDispatcher(t *testing.T) {
	assert := assert.New(t)
	defer viper.Reset()
	private, _, _ := test.GeneratePeerID(t)
	privateKey := test.MarshalPrivateKey(t, private)
	viper.Set("dispatcher.private_key", privateKey)
	viper.Set("dispatcher.listen_addr", "/ip4/0.0.0.0/tcp/0")
	viper.Set("dispatcher.database_connection_string", test.PostgresConnectionString)
	viper.Set("module.echo.enabled", true)
	dis, err := newDispatcher(context.TODO())
	assert.NotNil(dis)
	assert.Nil(err)
}
