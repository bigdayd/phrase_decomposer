package phrase_decomposer

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"github.com/steosofficial/steosmorphy/analyzer"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

type PhraseDecomposer struct {
	analyzer  *analyzer.MorphAnalyzer
	blackList []string
	custom    map[string]string
	adjDict   map[string]string
}

func NewPhraseDecomposer() *PhraseDecomposer {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				ClientSessionCache: tls.NewLRUClientSessionCache(2),
			},
			MaxIdleConns:        1,
			IdleConnTimeout:     30 * time.Second,
			MaxIdleConnsPerHost: 1,
			MaxConnsPerHost:     1,
		},
	}
	req, err := http.NewRequest("GET", "https://raw.githubusercontent.com/bigdayd/phrase_decomposer/refs/heads/master/adjToNoun_fixed.json", nil)
	if err != nil {
		panic(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	adjDict := make(map[string]string)
	err = decoder.Decode(&adjDict)
	if err != nil {
		panic(err)
	}

	a, err := analyzer.LoadMorphAnalyzer()
	if err != nil {
		panic(err)
	}
	return &PhraseDecomposer{
		a,
		[]string{
			"и", "в", "во", "на", "с", "со", "по",
			"от", "за", "над", "при", "об", "для", "это",
			"такие", "такой", "тот", "этот", "другой",
			"из", "до", "не", "как", "так", "под", "для",
			"размер", "тип", "цвет", "пол", "состав",
			"все",
		},
		map[string]string{
			"см":  "см",
			"мм":  "мм",
			"м":   "метр",
			"г":   "грамм",
			"кг":  "килограмм",
			"мл":  "миллилитр",
			"шт":  "штука",
			"хб":  "хлопок",
			"тв":  "телевизор",
			"usb": "usb",
			"гб":  "гигабайт",
		},
		adjDict,
	}
}

func (d *PhraseDecomposer) Decompose(phrase string) []string {
	phrase = strings.Replace(strings.ToLower(phrase), "ё", "е", -1)
	m1 := regexp.MustCompile(`[a-zа-я]+`)
	ms := m1.FindAllStringSubmatch(phrase, -1)
	results := make([]string, 0)
	for _, m := range ms {
		if r, ok := d.custom[m[0]]; ok {
			results = append(results, r)
			continue
		}

		word, err := d.normalizeWord(m[0])
		if err == nil {
			results = append(results, word)
		}
	}

	return results
}

func (d *PhraseDecomposer) normalizeWord(word string) (string, error) {
	if utf8.RuneCountInString(word) <= 2 {
		return "", errors.New("too short")
	}
	if slices.Contains(d.blackList, word) {
		return "", errors.New("black list")
	}

	if r, ok := d.custom[word]; ok {
		return r, nil
	}
	if slices.Contains(d.blackList, word) {
		return "", errors.New("black list")
	}

	parses := d.analyzer.Parse(word)
	if parses == nil || len(parses) == 0 {
		return word, nil
	}

	JSON, _ := json.MarshalIndent(parses, "", "  ")
	println("parses", string(JSON))

	for _, parse := range parses {
		if slices.Contains(d.blackList, parse.Lemma) || slices.Contains(d.blackList, parse.Word) {
			return "", errors.New("black list")
		}

		if parse.PartOfSpeech == "Наречие" || parse.PartOfSpeech == "Деепричастие" ||
			parse.PartOfSpeech == "Предлог" || parse.PartOfSpeech == "Частица" ||
			parse.PartOfSpeech == "Местоимение" || parse.PartOfSpeech == "Союз" {
			return "", errors.New("black list")
		} else if parse.PartOfSpeech == "Существительное" {
			if parse.Case == "Именительный" && parse.Number == "Единственное число" {
				return parse.Word, nil
			}
			forms := d.analyzer.Inflect(parse.Word)
			for _, form := range forms {
				if form.Case == "Именительный" && form.Number == "Единственное число" {
					return form.Word, nil
				}
			}
		} else if parse.PartOfSpeech == "Прилагательное" {
			if noun, ok := d.adjDict[parse.Lemma]; ok {
				return noun, nil
			}
			forms := d.analyzer.Inflect(parse.Word)
			for _, form := range forms {
				if form.PartOfSpeech == "Существительное" && form.Number == "Единственное число" {
					return form.Word, nil
				}
			}
		} else if parse.PartOfSpeech == "Глагол" || parse.PartOfSpeech == "Причастие" {
			return parse.Lemma, nil
		}
	}

	return parses[0].Lemma, nil
}
