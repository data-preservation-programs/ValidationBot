package echo

import (
	"encoding/json"

	"validation-bot/task"

	"gorm.io/gorm"
)

type Input struct {
	task.Task
	Input string `json:"input"`
}

func (i Input) Marshal() ([]byte, error) {
	return json.Marshal(i)
}

type ResultContent struct {
	task.Task
	Output string `json:"output"`
}

type EchoResult struct {
	gorm.Model
	ResultContent
}
