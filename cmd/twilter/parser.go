package main

import (
	"fmt"
	"github.com/kawasin73/twilter"
	"strings"
)

func unwrapArgs(args string) (string, error) {
	// remove ( and )
	if len(args) < 3 || args[0] != '(' || args[len(args)-1] != ')' {
		return "", fmt.Errorf("args not have ()")
	}
	return args[1 : len(args)-1], nil
}

func splitArgs(args string, sep string) ([]string, error) {
	if len(args) == 0 {
		// empty argument
		return nil, nil
	}

	var values []string
	var depth, head int
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case sep[0]:
			if depth == 0 {
				if args[i:i+len(sep)] == sep {
					// split argument
					values = append(values, args[head:i])
					i += len(sep) - 1
					head = i + 1
				}
			}

		case '(':
			depth++

		case ')':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("too many \")\" : \"%v\"", args[i:])
			}
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("too many \"(\"")
	}

	// add last argument
	values = append(values, args[head:])

	return values, nil
}

func parseFilters(value string, sep string) ([]twilter.Filter, error) {
	// parse multi filters separated by sep
	values, err := splitArgs(value, sep)
	if err != nil {
		return nil, err
	}
	var filters []twilter.Filter
	for _, v := range values {
		filter, err := parseFilter(v)
		if err != nil {
			return nil, err
		}
		filters = append(filters, filter)
	}
	return filters, nil
}

func parseFilter(value string) (twilter.Filter, error) {
	// parse each filter
	switch {
	case value == "photo":
		// "photo"
		return twilter.PhotoFilter{}, nil

	case value == "video":
		// "video"
		return twilter.VideoFilter{}, nil

	case value == "rt":
		// "rt"
		return twilter.RTFilter{}, nil

	case value == "qt":
		// "qt"
		return twilter.QTFilter{}, nil

	case strings.HasPrefix(value, "not"):
		// "not(<filter>)"
		args, err := unwrapArgs(value[3:])
		if err != nil {
			return nil, err
		}
		filter, err := parseFilter(args)
		if err != nil {
			return nil, err
		}
		return twilter.NotFilter{filter}, nil

	case strings.HasPrefix(value, "and"):
		// "and(<filter>[,<filter>[,...]])"
		if args, err := unwrapArgs(value[3:]); err != nil {
			return nil, err
		} else if filters, err := parseFilters(args, ","); err != nil {
			return nil, err
		} else {
			return twilter.AndFilter{filters}, nil
		}

	case strings.HasPrefix(value, "or"):
		// "or(<filter>[,<filter>[,...]])"
		if args, err := unwrapArgs(value[2:]); err != nil {
			return nil, err
		} else if filters, err := parseFilters(args, ","); err != nil {
			return nil, err
		} else {
			return twilter.OrFilter{filters}, nil
		}

	default:
		// filter is invalid
		return nil, fmt.Errorf("filter \"%v\" is invalid", value)
	}
}

// target is pair of screenName and filters.
type target struct {
	screenName string
	filters    []twilter.Filter
}

// targetValue stores targets.
type targetValue map[string]*target

// String returns string output.
func (tv targetValue) String() string {
	str := ""
	for name, t := range tv {
		str += fmt.Sprintf("%s:%v,", name, t.filters)
	}
	return str
}

// Set convert string to target and set or merge it in map.
func (tv targetValue) Set(value string) error {
	// get screen_name
	idx := strings.Index(value, ":")
	if idx < 0 {
		return fmt.Errorf("target has no screenName nor filter")
	}
	screenName := value[:idx]

	// get filters
	filters, err := parseFilters(value[idx+1:], "/")
	if err != nil {
		return err
	}

	// get target
	t, ok := tv[screenName]
	if !ok {
		// create new target
		t = &target{
			screenName: screenName,
		}
		tv[screenName] = t
	}

	// set filters
	t.filters = append(t.filters, filters...)

	return nil
}
