package models

import (
	"database/sql/driver"
	"errors"
	"fmt"
)

type JSONText []byte

func (j JSONText) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

func (j *JSONText) UnmarshalJSON(data []byte) error {
	if j == nil {
		return errors.New("models.JSONText: UnmarshalJSON on nil pointer")
	}
	*j = append((*j)[0:0], data...)
	return nil
}

func (j *JSONText) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		*j = nil
	case []byte:
		*j = append((*j)[0:0], v...)
	case string:
		*j = JSONText(v)
	default:
		return fmt.Errorf("models.JSONText: cannot scan %T", value)
	}
	return nil
}

func (j JSONText) Value() (driver.Value, error) {
	if len(j) == 0 {
		return []byte("{}"), nil
	}
	return []byte(j), nil
}
