package common

import (
	"strings"
	"testing"
)

func reconstructFromWrapped(result string) string {
	lines := strings.Split(result, "\n")
	var sb strings.Builder
	for i, line := range lines {
		if i > 0 {
			sb.WriteString(strings.TrimLeft(line, " "))
		} else {
			sb.WriteString(line)
		}
	}
	return sb.String()
}

func removeSpaces(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if r != ' ' {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func TestWriteWrappedStringNoCharacterLoss(t *testing.T) {
	testStrings := []string{
		"hello world",
		"This is a very long string that should be wrapped across multiple lines because it exceeds the maximum width",
		"word1 word2 word3 word4 word5 word6 word7 word8 word9 word10 word11 word12",
		"arn:aws:ec2:us-east-1:123456789012:instance/i-1234567890abcdef0",
		"/very/long/path/to/some/resource/that/needs/wrapping/across/lines",
		"item1,item2,item3,item4,item5,item6,item7,item8,item9,item10",
		"한글 테스트 문자열입니다. 이것은 매우 긴 한글 문자열로서 여러 줄에 걸쳐 표시되어야 합니다.",
		"これは非常に長い日本語の文字列で、複数の行に折り返す必要があります。",
		"这是一个很长的中文字符串，需要在多行之间换行显示。",
		"Hello 世界! This is a mixed string with 한글 and 日本語 characters.",
		"🚀 Launch 🎉 Party 🌟 Stars 🎊 Celebration 🎁 Gifts 🎈 Balloons",
		"supercalifragilisticexpialidocioussupercalifragilisticexpialidocious",
		"123456789012345678901234567890",
		`"This is a \"quoted\" string with\nnewlines and\ttabs inside"`,
		"This string must wrap even with minimum width",
		"sg-0123456789abcdef0,sg-abcdef0123456789a,sg-fedcba9876543210f",
		"i-0123456789abcdef0 (running) - t3.xlarge - us-east-1a",
		"https://console.aws.amazon.com/ec2/home?region=us-east-1#Instances:instanceId=i-0123456789abcdef0",
		"a b c d e f g h i j k l m n o p q r s t u v w x y z",
		"test-with-many-hyphens-in-the-string-that-could-break",
		"path/to/file/with/many/slashes/in/it/test",
	}

	for _, s := range testStrings {
		for width := 22; width <= 120; width++ {
			for indent := 0; indent <= 20; indent += 4 {
				var sb strings.Builder
				writeWrappedString(&sb, s, indent, width)
				result := sb.String()

				reconstructed := reconstructFromWrapped(result)
				want := removeSpaces(s)
				got := removeSpaces(reconstructed)

				if got != want {
					t.Errorf("CHAR LOSS: width=%d indent=%d", width, indent)
					t.Errorf("  input:         %q", s)
					t.Errorf("  output:        %q", result)
					t.Errorf("  reconstructed: %q", reconstructed)
					t.Errorf("  want (no sp):  %q", want)
					t.Errorf("  got  (no sp):  %q", got)
					return
				}
			}
		}
	}
}

func TestWriteWrappedStringZeroWidth(t *testing.T) {
	var sb strings.Builder
	input := "This should not be wrapped at all"
	writeWrappedString(&sb, input, 0, 0)
	if sb.String() != input {
		t.Errorf("zero width should return input unchanged, got %q", sb.String())
	}
}

func TestWriteWrappedStringNegativeWidth(t *testing.T) {
	var sb strings.Builder
	input := "This should not be wrapped at all"
	writeWrappedString(&sb, input, 0, -10)
	if sb.String() != input {
		t.Errorf("negative width should return input unchanged, got %q", sb.String())
	}
}

func TestWriteWrappedStringWithPrefix(t *testing.T) {
	testStrings := []string{
		"arn:aws:ec2:us-east-1:123456789012:instance/i-1234567890abcdef0",
		"https://console.aws.amazon.com/ec2/home?region=us-east-1#Instances",
		"sg-0123456789abcdef0,sg-abcdef0123456789a,sg-fedcba9876543210f",
	}

	prefixes := []string{
		"key: ",
		"instance_id: ",
		"security_groups: ",
		"very_long_key_name_here: ",
	}

	for _, prefix := range prefixes {
		for _, s := range testStrings {
			for width := 30; width <= 100; width += 5 {
				var sb strings.Builder
				sb.WriteString(prefix)
				writeWrappedString(&sb, s, 0, width)
				result := sb.String()

				resultWithoutPrefix := result[len(prefix):]
				reconstructed := reconstructFromWrapped(resultWithoutPrefix)
				want := removeSpaces(s)
				got := removeSpaces(reconstructed)

				if got != want {
					t.Errorf("CHAR LOSS with prefix: width=%d prefix=%q", width, prefix)
					t.Errorf("  input:         %q", s)
					t.Errorf("  full output:   %q", result)
					t.Errorf("  want (no sp):  %q", want)
					t.Errorf("  got  (no sp):  %q", got)
					return
				}
			}
		}
	}
}
