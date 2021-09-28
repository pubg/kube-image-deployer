package util

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var ErrNotFound = errors.New("not found")

// GetHighestVersionWithFilter versions는 version 목록이다.
// filter는 *(asterisk)를 숫자(\d+)로 대입하는 regexp로 변환된다.
// 예를 들어, filter가 "1.2.*"이면, 1.2.3이나 1.2.4라는 버전을 찾는다.
// 그리고 그 중 *의 위치에 해당하는 숫자가 가장 큰 버전을 반환한다.
// *은 여럿일 수 있으며 왼쪽에서 오른쪽으로 동일 위치에서 더 큰 버전을 찾는다.
// 예를 들어, filter가 "1.*.*"이면, 1.1.5와 1.2.0 중 1.2.0을 반환한다.
func GetHighestVersionWithFilter(versions []string, filter string) (string, error) {
	highestTag := ""
	highestNumbers := []int64{}
	regexString := fmt.Sprintf("^%s$", strings.Replace(regexp.QuoteMeta(filter), `\*`, `(\d+)`, -1))
	patt, err := regexp.Compile(regexString)

	if nil != err {
		return "", err
	}

	for _, tag := range versions {
		matches := patt.FindStringSubmatch(tag)

		if tag == "" || len(matches) < 2 {
			continue
		}

		numbers := []int64{}

		for idx, match := range matches {
			if idx == 0 { // 첫번째 매치는 무시한다.
				continue
			}
			if number, err := strconv.ParseInt(match, 10, 64); err != nil {
				return "", err
			} else {
				numbers = append(numbers, number)
			}
		}

		for idx, number := range numbers { // 각 자릿수를 비교해 더 큰 값이 발견되면 교체
			if idx >= len(highestNumbers) || highestNumbers[idx] <= number {
				highestTag, highestNumbers = tag, numbers
				break
			}
		}
	}

	if highestTag == "" {
		return "", ErrNotFound
	}

	return highestTag, nil
}
