package main

import (
	"regexp"
	"fmt"
	"strings"
	"reflect"
)

const tagName = "validate" // Name of the struct tag used for validation

var valuesRegex = regexp.MustCompile("^values=([a-zA-Z0-9|]+)$")

// Generic data validator
type Validator interface {
	// Validate method performs validation and returns result and optional error
	Validate(interface{}) (bool, error)
}

// DefaultValidator does not perform any validations
type DefaultValidator struct {
}

func (v DefaultValidator) Validate(val interface{}) (bool, error) {
	return true, nil
}

// BoolValidator validates string presence and/or its length.
type BoolValidator struct {
	Required bool
	Values   string
}

func (v BoolValidator) Validate(val interface{}) (bool, error) {
	value := val.(string)
	l := len(value)

	if v.Required && l == 0 {
		return false, fmt.Errorf("cannot be blank")
	}

	if v.Values != "" {
		values := strings.Split(v.Values, "|")
		if !contains(values, value) {
			return false, fmt.Errorf("has illegal value")
		}
	}

	return true, nil
}

// StringValidator validates string presence and/or its length.
type StringValidator struct {
	Required bool
	Values   string
}

func (v StringValidator) Validate(val interface{}) (bool, error) {
	value := val.(string)
	l := len(value)

	if v.Required && l == 0 {
		return false, fmt.Errorf("cannot be blank")
	}

	if v.Values != "" {
		values := strings.Split(v.Values, "|")
		if !contains(values, value) {
			return false, fmt.Errorf("has illegal value")
		}
	}

	return true, nil
}

// NumberValidator performs numerical value validation.
type NumberValidator struct {
	Min int
	Max int
}

func (v NumberValidator) Validate(val interface{}) (bool, error) {
	num := val.(int)

	if num < v.Min {
		return false, fmt.Errorf("should be greater than %v", v.Min)
	}

	if v.Max >= v.Min && num > v.Max {
		return false, fmt.Errorf("should be less than %v", v.Max)
	}

	return true, nil
}

// EmailValidator checks if string is a valid email address.
type EmailValidator struct {
	Required bool
}

func (v EmailValidator) Validate(val interface{}) (bool, error) {
	value := val.(string)
	l := len(value)

	if v.Required && l == 0 {
		return false, fmt.Errorf("cannot be blank")
	}

	const emailRegexString = "^(?:(?:(?:(?:[a-zA-Z]|\\d|[!#\\$%&'\\*\\+\\-\\/=\\?\\^_`{\\|}~]|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])+(?:\\.([a-zA-Z]|\\d|[!#\\$%&'\\*\\+\\-\\/=\\?\\^_`{\\|}~]|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])+)*)|(?:(?:\\x22)(?:(?:(?:(?:\\x20|\\x09)*(?:\\x0d\\x0a))?(?:\\x20|\\x09)+)?(?:(?:[\\x01-\\x08\\x0b\\x0c\\x0e-\\x1f\\x7f]|\\x21|[\\x23-\\x5b]|[\\x5d-\\x7e]|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])|(?:\\(?:[\\x01-\\x09\\x0b\\x0c\\x0d-\\x7f]|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}]))))*(?:(?:(?:\\x20|\\x09)*(?:\\x0d\\x0a))?(\\x20|\\x09)+)?(?:\\x22)))@(?:(?:(?:[a-zA-Z]|\\d|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])|(?:(?:[a-zA-Z]|\\d|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])(?:[a-zA-Z]|\\d|-|\\.|_|~|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])*(?:[a-zA-Z]|\\d|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])))\\.)+(?:(?:[a-zA-Z]|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])|(?:(?:[a-zA-Z]|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])(?:[a-zA-Z]|\\d|-|\\.|_|~|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])*(?:[a-zA-Z]|[\\x{00A0}-\\x{D7FF}\\x{F900}-\\x{FDCF}\\x{FDF0}-\\x{FFEF}])))\\.?$"
	var emailRegex = regexp.MustCompile(emailRegexString)

	if !emailRegex.MatchString(value) {
		return false, fmt.Errorf("is not a valid email address")
	}

	return true, nil
}

// Returns validator struct corresponding to validation type
func getValidatorFromTag(tag string) Validator {
	args := strings.Split(tag, ",")

	switch args[0] {
	case "number":
		validator := NumberValidator{}
		fmt.Sscanf(strings.Join(args[1:], ","), "min=%d,max=%d", &validator.Min, &validator.Max)
		return validator
	case "bool":
		validator := BoolValidator{}
		for _, flag := range args[1:] {
			if string(flag) == "required" {
				validator.Required = true
			} else {
				results := valuesRegex.FindStringSubmatch(string(flag))
				if len(results) > 0 {
					validator.Values = results[1]
				}
			}
		}
		return validator
	case "string":
		validator := StringValidator{}
		for _, flag := range args[1:] {
			if string(flag) == "required" {
				validator.Required = true
			} else {
				results := valuesRegex.FindStringSubmatch(string(flag))
				if len(results) > 0 {
					validator.Values = results[1]
				}
			}
		}
		return validator
	case "email":
		validator := EmailValidator{}
		if contains(args[1:], "required") {
			validator.Required = true
		}
		return validator
	default:
		return DefaultValidator{}
	}
}

//================================================

// Performs actual data validation using validator definitions on the struct
func validateStruct(s interface{}) (found bool, errs []string) {

	found = false

	// ValueOf returns a Value representing the run-time data
	v := reflect.ValueOf(s)

	for i := 0; i < v.NumField(); i++ {
		// Get the field tag value
		tag := v.Type().Field(i).Tag.Get(tagName)

		// Skip if tag is not defined or ignored
		if tag == "" || tag == "-" {
			continue
		}

		// Get a validator that corresponds to a tag
		validator := getValidatorFromTag(tag)

		// Perform validation
		valid, err := validator.Validate(v.Field(i).Interface())

		// Append error to results
		if !valid && err != nil {
			found = true
			errs = append(errs, fmt.Sprintf("%s %s", v.Type().Field(i).Name, err.Error()))
		}
	}

	return
}
