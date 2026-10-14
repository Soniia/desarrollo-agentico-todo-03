package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gofrs/uuid"
)

// TitleMaxLength es el número máximo de caracteres permitido en el título.
const TitleMaxLength = 200

// Valores posibles de la prioridad de una tarea.
const (
	PriorityHigh   = "alta"
	PriorityMedium = "media"
	PriorityLow    = "baja"
)

// Priorities enumera las prioridades en el orden en que se muestran.
var Priorities = []string{PriorityHigh, PriorityMedium, PriorityLow}

// PriorityLabel devuelve el texto visible de una prioridad, o "" si no es válida.
func PriorityLabel(p string) string {
	switch p {
	case PriorityHigh:
		return "Alta"
	case PriorityMedium:
		return "Media"
	case PriorityLow:
		return "Baja"
	}
	return ""
}

// Task representa una tarea de la lista.
type Task struct {
	ID        uuid.UUID `json:"id" db:"id" form:"-"`
	Title     string    `json:"title" db:"title" form:"title"`
	Completed bool      `json:"completed" db:"completed" form:"-"`
	Priority  string    `json:"priority" db:"priority" form:"priority"`
	CreatedAt time.Time `json:"created_at" db:"created_at" form:"-"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" form:"-"`
}

// Tasks es una colección de Task.
type Tasks []Task

// PriorityLabel devuelve el texto visible de la prioridad de la tarea.
func (t Task) PriorityLabel() string {
	return PriorityLabel(t.Priority)
}

// BeforeValidate asigna la prioridad media cuando no se ha indicado ninguna.
func (t *Task) BeforeValidate(tx *pop.Connection) error {
	if strings.TrimSpace(t.Priority) == "" {
		t.Priority = PriorityMedium
	}
	return nil
}

// Validate se ejecuta en cada llamada a los métodos "pop.Validate*".
func (t *Task) Validate(tx *pop.Connection) (*validate.Errors, error) {
	verrs := validate.NewErrors()

	title := strings.TrimSpace(t.Title)
	if title == "" {
		verrs.Add("title", "El título es obligatorio.")
	} else if len([]rune(title)) > TitleMaxLength {
		verrs.Add("title", fmt.Sprintf("El título no puede superar los %d caracteres.", TitleMaxLength))
	}

	if PriorityLabel(t.Priority) == "" {
		verrs.Add("priority", "La prioridad no es válida.")
	}

	return verrs, nil
}
