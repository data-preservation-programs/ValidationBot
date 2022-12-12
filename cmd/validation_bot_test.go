package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	mock2 "validation-bot/cmd/mock"

	"validation-bot/task"

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
 * 3. Create database helper
 */
const (
	testPeerId = "12D3KooWG8tR9PHjjXcMknbNPVWT75BuXXA2RaYx3fMwwg2oPZXd"
	testUuid   = "d54ec3a8-4045-11ed-b878-0242ac120002"
)

func TestDeleteTaskHandler(t *testing.T) {
	assert := assert.New(t)
	defer viper.Reset()
	mockTaskRemover := new(mock2.MockTaskRemover)
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
	assert := assert.New(t)
	defer viper.Reset()
	taskDef := task.Definition{}
	taskDef.Definition.Set(`{"helper": "helper"}`)
	mockTaskCreator := new(mock2.MockTaskCreator)
	mockTaskCreator.On("Create", mock.Anything, mock.Anything).Return(nil)
	e := echo.New()
	buf := bytes.NewBuffer([]byte(`{"definition":{"helper": "helper"}}`))
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
	assert := assert.New(t)
	defer viper.Reset()
	mockTaskCreator := new(mock2.MockTaskCreator)
	taskDef := task.Definition{}
	taskDef.Definition.Set(`{"helper": "helper"}`)
	taskDefs := []task.Definition{
		taskDef,
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
	assert.Equal(
		"[{\"id\":\"00000000-0000-0000-0000-000000000000\",\"target\":\"\",\"type\":\"\",\"intervalSeconds\":0,\"definition\":{\"helper\":\"helper\"},\"tag\":\"\",\"DispatchedTimes\":0,\"CreatedAt\":\"0001-01-01T00:00:00Z\",\"UpdatedAt\":\"0001-01-01T00:00:00Z\"}]\n",
		rec.Body.String(),
	)
}
