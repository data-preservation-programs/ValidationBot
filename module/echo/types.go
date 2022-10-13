package echo

import (
	"validation-bot/task"

	"gorm.io/gorm"
)

type Input struct {
	task.Task
	Input string `json:"input"`
}

type Result struct {
	task.Task
	Output string `json:"output"`
}

type ResultModel struct {
	gorm.Model
	Result
}

func (ResultModel) TableName() string {
	return "echo_results"
}
