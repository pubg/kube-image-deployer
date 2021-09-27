package util

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var ErrNotFound = errors.New("not found")

func GetHighestVersionWithFilter(versions []string, filter string) (string, error) {
	targetTag := ""
	targetVer := int64(0)

	patt, err := regexp.Compile(fmt.Sprintf("^%s$", strings.Replace(regexp.QuoteMeta(filter), "\\*", "(\\d+)", 1)))
	if nil != err {
		return "", err
	}

	for _, v := range versions {
		// fmt.Println(v)
		matches := patt.FindStringSubmatch(v)

		if len(matches) == 0 {
			continue
		}

		ver, err := strconv.ParseInt(matches[1], 10, 64)
		if nil != err {
			continue
		}

		if targetVer > ver {
			continue
		}

		targetTag, targetVer = v, ver
	}

	if targetTag == "" {
		return "", ErrNotFound
	}

	return targetTag, nil
}
