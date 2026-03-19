package utils

import "github.com/google/jsonschema-go/jsonschema"

func DescribeJSONSchema[T any]() (string, error) {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		return "", err
	}
	schemaJSON, err := schema.MarshalJSON()
	if err != nil {
		return "", err
	}
	return string(schemaJSON), nil
}
