package main

import "fmt"
import "time"
import "regexp"
import "strconv"
import "strings"

import "github.com/prataprc/goparsec"

type Voucher interface {
	ToLedger() []string
}

func newvoucher(fields ...parsec.ParsecNode) Voucher {
	switch fields[4].(string) {
	case "Jrnl":
		return NewJournal(fields...)
	case "Rcpt":
		return NewReceipt(fields...)
	case "Pymt":
		return NewPayment(fields...)
	case "Ctra":
		return NewContra(fields...)
	default:
		panic(fmt.Errorf("unsupported voucher type : %s", fields[4]))
	}
	return nil
}

func formatfields(fields ...parsec.ParsecNode) string {
	items := []string{}
	for i, field := range fields {
		items = append(items, fmt.Sprintf("%v:%v", i, field))
	}
	return strings.Join(items, ", ")
}

type Receipt struct {
	Date     time.Time
	Debtor   string
	Creditor string
	Debit    float64
	Credit   float64
	Notes    []string
	parsed   bool
}

func NewReceipt(fields ...parsec.ParsecNode) (r *Receipt) {
	notesorignore := []int{2, 3, 5, 7, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18,
		19, 20, 21}
	if fields[4].(string) != "Rcpt" {
		panic("impossible situation")
	}

	r = &Receipt{
		Date:     parsetime(fields[0]),
		Debtor:   fields[8].(string),
		Creditor: fields[1].(string),
		Debit:    parsefloat(fields[6]),
		Credit:   -parsefloat(fields[6]),
		Notes:    getnotes(fields, notesorignore),
		parsed:   true,
	}
	return r
}

func (r *Receipt) ToLedger() []string {
	tm := r.Date.Format("2006-01-02")
	lines := []string{
		fmt.Sprintf("%v  %v", tm, "Payee"),
		fmt.Sprintf("    %-40v  %.2f", r.Debtor, r.Debit),
		fmt.Sprintf("    %-40v  %.2f", r.Creditor, r.Credit),
		fmt.Sprintf("    ; Receipt"),
	}
	for _, note := range r.Notes {
		lines = append(lines, fmt.Sprintf("    ; %v", note))
	}
	return lines
}

type Journal struct {
	Date     time.Time
	Debtor   string
	Creditor string
	Debit    float64
	Credit   float64
	Notes    []string
	parsed   bool
}

func NewJournal(fields ...parsec.ParsecNode) (j *Journal) {
	notesorignore := []int{2, 3, 6, 7, 8, 11, 13, 15, 16, 19, 20, 21}
	if fields[4].(string) != "Jrnl" {
		panic("impossible situation")
	}

	creditor := strings.Trim(fields[12].(string), " ")
	if creditor == "" {
		creditor = strings.Trim(fields[8].(string), " ")
	}
	j = &Journal{
		Date:     parsetime(fields[0]),
		Debtor:   fields[1].(string),
		Creditor: creditor,
		Debit:    -parsefloat(fields[5]),
		Credit:   parsefloat(fields[5]),
		Notes:    getnotes(fields, notesorignore),
		parsed:   true,
	}
	return j
}

func (j *Journal) ToLedger() []string {
	tm := j.Date.Format("2006-01-02")
	lines := []string{
		fmt.Sprintf("%v  %v", tm, "Payee"),
		fmt.Sprintf("    %-40v  %.2f", j.Debtor, j.Debit),
		fmt.Sprintf("    %-40v  %.2f", j.Creditor, j.Credit),
		fmt.Sprintf("    ; Journal"),
	}
	for _, note := range j.Notes {
		lines = append(lines, fmt.Sprintf("    ; %v", note))
	}
	return lines
}

type Payment struct {
	Date     time.Time
	Debtor   string
	Creditor string
	Debit    float64
	Credit   float64
	Notes    []string
	parsed   bool
}

func NewPayment(fields ...parsec.ParsecNode) (p *Payment) {
	notesorignore := []int{2, 3, 6, 7, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18,
		19, 20, 21}
	if fields[4].(string) != "Pymt" {
		panic("impossible situation")
	}

	p = &Payment{
		Date:     parsetime(fields[0]),
		Debtor:   fields[1].(string),
		Creditor: fields[8].(string),
		Debit:    -parsefloat(fields[5]),
		Credit:   parsefloat(fields[5]),
		Notes:    getnotes(fields, notesorignore),
		parsed:   true,
	}
	return p
}

func (p *Payment) ToLedger() []string {
	tm := p.Date.Format("2006-01-02")
	lines := []string{
		fmt.Sprintf("%v  %v", tm, "Payee"),
		fmt.Sprintf("    %-40v  %.2f", p.Debtor, p.Debit),
		fmt.Sprintf("    %-40v  %.2f", p.Creditor, p.Credit),
		fmt.Sprintf("    ; Payment"),
	}
	for _, note := range p.Notes {
		lines = append(lines, fmt.Sprintf("    ; %v", note))
	}
	return lines
}

type Contra struct {
	Date     time.Time
	Debtor   string
	Creditor string
	Debit    float64
	Credit   float64
	Notes    []string
	parsed   bool
}

func NewContra(fields ...parsec.ParsecNode) (c *Contra) {
	notesorignore := []int{2, 3, 5, 7, 8, 9, 10, 11, 13, 14, 15, 16, 17, 18,
		19, 20, 21}
	if fields[4].(string) != "Ctra" {
		panic("impossible situation")
	}

	c = &Contra{
		Date:     parsetime(fields[0]),
		Debtor:   fields[1].(string),
		Creditor: fields[12].(string),
		Debit:    parsefloat(fields[6]),
		Credit:   -parsefloat(fields[6]),
		Notes:    getnotes(fields, notesorignore),
		parsed:   true,
	}
	return c
}

func (c *Contra) ToLedger() []string {
	tm := c.Date.Format("2006-01-02")
	lines := []string{
		fmt.Sprintf("%v  %v", tm, "Payee"),
		fmt.Sprintf("    %-40v  %.2f", c.Debtor, c.Debit),
		fmt.Sprintf("    %-40v  %.2f", c.Creditor, c.Credit),
		fmt.Sprintf("    ; Contra"),
	}
	for _, note := range c.Notes {
		lines = append(lines, fmt.Sprintf("    ; %v", note))
	}
	return lines
}

func getnotes(fields []parsec.ParsecNode, indexes []int) []string {
	notes := []string{}
	for _, index := range indexes {
		if index >= len(fields) {
			continue
		}
		if s, ok := fields[index].(string); ok {
			if s != "" {
				notes = append(notes, s)
			}
			continue
		} else if _, ok := fields[index].(parsec.MaybeNone); ok {
			continue
		}
	}
	return notes
}

func parsefloat(field parsec.ParsecNode) float64 {
	t := field.(*parsec.Terminal)
	credit, err := strconv.ParseFloat(string(t.Value), 64)
	if err != nil {
		panic(fmt.Errorf("unable to parse amount: %v", string(t.Value)))
	}
	return credit
}

func parsetime(field parsec.ParsecNode) time.Time {
	t := field.(*parsec.Terminal)
	re, _ := regexp.Compile("([0-9]{1,2})-([0-9]{1,2})-([0-9]{4})")
	matches := re.FindStringSubmatch(string(t.Value))
	date, _ := strconv.Atoi(matches[1])
	month, _ := strconv.Atoi(matches[2])
	year, _ := strconv.Atoi(matches[3])
	return time.Date(year, time.Month(month), date, 0, 0, 0, 0, time.Local)
}
