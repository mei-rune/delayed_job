package delayed_job

import (
	"errors"
	"strconv"
	"strings"
)

var invalidIndex = errors.New("invalid index")
var indexOutOfRange = errors.New("index out of range")
var valueIsNilInResult = errors.New("value is nil in the result")
var isNull = errors.New("value is nil")
var notFound = errors.New("not found")
var notImplemented = errors.New("not implemented")
var notMap = errors.New("it is not a map")
var notArray = errors.New("it is not a array")

type valueHolder struct {
	value            interface{}
	object_converted bool
	object_s2i       map[string]interface{}
	object_s2s       map[string]string

	array_converted bool
	array_if        []interface{}
	array_s2i       []map[string]interface{}
	array_s2s       []map[string]string
}

func newValueHolder(v interface{}) *valueHolder {
	return &valueHolder{value: v}
}

func (self *valueHolder) attribute(name string) (interface{}, error) {
	if !self.object_converted {
		self.object_converted = true

		switch m := self.value.(type) {
		case map[string]interface{}:
			self.object_s2i = m
			if nil == self.object_s2i {
				return nil, isNull
			}
			goto string2interface
		case map[string]string:
			self.object_s2s = m
			if nil == self.object_s2s {
				return nil, isNull
			}
			goto string2string
		default:
			return nil, notMap
		}
	} else {
		if nil != self.object_s2i {
			goto string2interface
		}
		if nil != self.object_s2s {
			goto string2string
		}
		return nil, isNull
	}

string2interface:
	if v, ok := self.object_s2i[name]; ok {
		return v, nil
	}
	return nil, notFound

string2string:
	if s, ok := self.object_s2s[name]; ok {
		return s, nil
	}
	return nil, notFound
}

func (self *valueHolder) array(idx int) (interface{}, error) {
	return self.array_attribute(idx, "")
}

func (self *valueHolder) array_attribute(idx int, name string) (interface{}, error) {
	if 0 > idx {
		return nil, invalidIndex
	}

	var obj interface{}
	var s2i map[string]interface{}
	var s2s map[string]string

	if !self.array_converted {
		self.array_converted = true

		switch m := self.value.(type) {
		case []interface{}:
			self.array_if = m
			if nil == self.array_if {
				return nil, isNull
			}
			goto array_interface
		case []map[string]interface{}:
			self.array_s2i = m
			if nil == self.array_s2i {
				return nil, isNull
			}
			goto array_string2interface
		case []map[string]string:
			self.array_s2s = m
			if nil == self.array_s2s {
				return nil, isNull
			}
			goto array_string2string
		default:
			return nil, notMap
		}
	} else {
		if nil != self.array_if {
			goto array_interface
		}
		if nil != self.array_s2i {
			goto array_string2interface
		}
		if nil != self.array_s2s {
			goto array_string2string
		}
		return nil, isNull
	}

array_interface:
	if idx >= len(self.array_if) {
		return nil, invalidIndex
	}
	obj = self.array_if[idx]
	if 0 == len(name) {
		return obj, nil
	}
	if nil == obj {
		return nil, isNull
	}
	switch m := obj.(type) {
	case map[string]interface{}:
		s2i = m
		if nil == s2i {
			return nil, isNull
		}
		if v, ok := s2i[name]; ok {
			return v, nil
		}
		return nil, notFound
	case map[string]string:
		s2s = m
		if nil == s2s {
			return nil, isNull
		}
		if v, ok := s2s[name]; ok {
			return v, nil
		}
		return nil, notFound
	default:
		return nil, notMap
	}

array_string2interface:
	if idx >= len(self.array_s2i) {
		return nil, invalidIndex
	}

	s2i = self.array_s2i[idx]
	if 0 == len(name) {
		return s2i, nil
	}
	if nil == s2i {
		return nil, isNull
	}
	if v, ok := s2i[name]; ok {
		return v, nil
	}
	return nil, notFound
array_string2string:
	if idx >= len(self.array_s2s) {
		return nil, invalidIndex
	}
	s2s = self.array_s2s[idx]
	if 0 == len(name) {
		return s2s, nil
	}
	if nil == s2s {
		return nil, isNull
	}
	if v, ok := s2s[name]; ok {
		return v, nil
	}
	return nil, notFound
}

// func (self *valueHolder) where(expression string) ([]interface{}, error) {
// 	return nil, notImplemented
// }

// func (self *valueHolder) whereOne(expression string) (interface{}, error) {
// 	i, e := strconv.ParseInt(expression, 10, 0)
// 	if nil != e {
// 		return nil, e
// 	}
// 	if i < 0 {
// 		return nil, invalidIndex
// 	}
// 	idx := int(i)

// 	if array, ok := value.([]interface{}); ok {
// 		if idx < len(array) {
// 			return array[idx], nil
// 		}
// 		return nil, indexOutOfRange
// 	} else if array, ok := value.([]map[string]interface{}); ok {
// 		if idx < len(array) {
// 			return array[idx], nil
// 		}
// 		return nil, indexOutOfRange
// 	} else if array, ok := value.([]map[string]string); ok {
// 		if idx < len(array) {
// 			return array[idx], nil
// 		}
// 		return nil, indexOutOfRange
// 	}
// 	return nil, fmt.Errorf("it is not a slice, actual is &T", value)
// }

func (self *valueHolder) simpleValue(attribute string) (interface{}, error) {
	if nil == self.value {
		return nil, isNull
	}

	if 0 == len(attribute) {
		return self.value, nil
	}

	if '[' != attribute[0] {
		return self.attribute(attribute)
	}

	idx := strings.IndexRune(attribute, ']')
	if -1 == idx {
		return nil, errors.New("sytex error: '" + attribute + "'")
	}

	array_idx, e := strconv.ParseInt(attribute[1:idx], 10, 0)
	if nil != e {
		return nil, errors.New("index is not a number, " + e.Error())
	}

	if (idx + 1) == len(attribute) {
		return self.array(int(array_idx))
	}

	if '.' != attribute[idx+1] || (idx+2) == len(attribute) {
		return nil, errors.New("sytex error: '" + attribute + "'")
	}

	return self.array_attribute(int(array_idx), attribute[idx+2:])
}
