package config

import (
	"errors"
	"log"
	"strings"
)

// skippedFields names theme fields and element properties this version of
// gopdf does not know. A theme is applied without them rather than
// refused, so that one written for a newer version still loads; the
// fields are reported as a warning.
type skippedFields struct{ names []string }

func (e *skippedFields) Error() string {
	return "skipped unknown theme fields: " + strings.Join(e.names, ", ")
}

func skipField(name string) error { return &skippedFields{names: []string{name}} }

func isSkipped(err error) bool {
	var skipped *skippedFields
	return errors.As(err, &skipped)
}

// skips gathers the fields skipped while applying a table.
type skips struct{ names []string }

// add notes err's skipped fields, returning any other error.
func (s *skips) add(err error) error {
	var skipped *skippedFields
	if errors.As(err, &skipped) {
		s.names = append(s.names, skipped.names...)
		return nil
	}
	return err
}

// err is the fields skipped, or nil.
func (s *skips) err() error {
	if len(s.names) == 0 {
		return nil
	}
	return &skippedFields{names: s.names}
}

// warn logs a problem the configuration is applied in spite of, and keeps
// it for the viewer to show.
func (r *Runtime) warn(err error) {
	log.Printf("config: %v", err)
	r.warnings = append(r.warnings, err.Error())
}

// TakeWarnings returns the warnings since it was last called.
func (r *Runtime) TakeWarnings() []string {
	if r == nil {
		return nil
	}
	warnings := r.warnings
	r.warnings = nil
	return warnings
}
