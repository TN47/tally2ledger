package main

import "fmt"
import "time"
import "regexp"
import "strconv"
import "strings"

import "github.com/prataprc/goparsec"
import "github.com/bnclabs/golog"

type Voucher interface {
	Type() string

	Rewrite(rules map[string]interface{})

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

var nreceipts int

type Receipt struct {
	Date      time.Time
	Payee     string
	Debtors   []string
	Creditors []string
	Debits    []float64
	Credits   []float64
	Notes     []string
	parsed    bool
}

func NewReceipt(fields ...parsec.ParsecNode) (r *Receipt) {
	nreceipts++

	if fields[4].(string) != "Rcpt" {
		panic("impossible situation")
	}

	debtors, debits := []string{}, []float64{}
	creditors, credits := []string{}, []float64{}
	// gather fields
	date, creditor, credit, err := parsefirstblock(fields, 6)
	if err != nil {
		log.Errorf("%v rcpt %v", njournals, err)
	}
	creditors, credits = append(creditors, creditor), append(credits, -credit)

	// partition
	offsets, err := dcpartition(fields)
	if err != nil {
		log.Errorf("%v dcpartition: %v\n", njournals, err)
		return nil
	}
	//fmt.Println(nreceipts, offsets)
	for _, off := range offsets {
		name, amount, err := parseposting(fields[off:])
		if err != nil {
			log.Errorf("%v receipt parse debit posting %v\n", nreceipts, err)
			return nil
		}
		if amount < 0 {
			debtors = append(debtors, name)
			debits = append(debits, -amount)
		} else {
			creditors = append(creditors, name)
			credits = append(credits, -amount)
		}
	}
	// gather notes
	notes := getfnotes(fields, offsets)

	r = &Receipt{
		Date: date, Debtors: debtors, Creditors: creditors,
		Debits: debits, Credits: credits, parsed: true,
		Notes: notes,
	}
	return r
}

func (r *Receipt) Type() string {
	return "Receipt"
}

func (r *Receipt) Rewrite(rules map[string]interface{}) {
	r.Debtors = rewritedebtors(rules, r.Debtors)
	r.Creditors = rewritecreditors(rules, r.Creditors)
	r.Payee = rewritepayee(rules, r.Payee)
}

func (r *Receipt) ToLedger() []string {
	tm := r.Date.Format("2006-01-02")
	lines := []string{
		fmt.Sprintf("%v  %v ; Receipt", tm, "Payee"),
	}
	for i, debtor := range r.Debtors {
		line := fmt.Sprintf("    %-40v  %.2f", debtor, r.Debits[i])
		lines = append(lines, line)
	}
	for i, creditor := range r.Creditors {
		line := fmt.Sprintf("    %-40v  %.2f", creditor, r.Credits[i])
		lines = append(lines, line)
	}
	for _, note := range r.Notes {
		lines = append(lines, fmt.Sprintf("    ; %v", note))
	}
	return lines
}

var njournals int

type Journal struct {
	Date      time.Time
	Payee     string
	Debtors   []string
	Creditors []string
	Debits    []float64
	Credits   []float64
	Notes     []string
	parsed    bool
}

func NewJournal(fields ...parsec.ParsecNode) (j *Journal) {
	njournals++

	if fields[4].(string) != "Jrnl" {
		panic("impossible situation")
	}

	debtors, debits := []string{}, []float64{}
	creditors, credits := []string{}, []float64{}
	// gather fields
	date, debtor, debit, err := parsefirstblock(fields, 5)
	if err != nil {
		log.Errorf("%v jrnl %v", njournals, err)
	}
	debtors, debits = append(debtors, debtor), append(debits, -debit)

	// partition
	offsets, err := dcpartition(fields)
	if err != nil {
		log.Errorf("%v dcpartition: %v\n", njournals, err)
		return nil
	}
	for _, off := range offsets {
		name, amount, err := parseposting(fields[off:])
		if err != nil {
			log.Errorf("%v journal parse debit posting %v\n", njournals, err)
			return nil
		}
		if amount < 0 {
			debtors = append(debtors, name)
			debits = append(debits, -amount)
		} else {
			creditors = append(creditors, name)
			credits = append(credits, -amount)
		}
	}
	// gather notes
	notes := getfnotes(fields, offsets)

	j = &Journal{
		Date: date, Debtors: debtors, Creditors: creditors,
		Debits: debits, Credits: credits, parsed: true,
		Notes: notes,
	}
	return j
}

func (j *Journal) Type() string {
	return "Journal"
}

func (j *Journal) Rewrite(rules map[string]interface{}) {
	j.Debtors = rewritedebtors(rules, j.Debtors)
	j.Creditors = rewritecreditors(rules, j.Creditors)
	j.Payee = rewritepayee(rules, j.Payee)
}

func (j *Journal) ToLedger() []string {
	tm := j.Date.Format("2006-01-02")
	lines := []string{
		fmt.Sprintf("%v  %v ; Journal", tm, "Payee"),
	}
	for i, debtor := range j.Debtors {
		line := fmt.Sprintf("    %-40v  %.2f", debtor, j.Debits[i])
		lines = append(lines, line)
	}
	for i, creditor := range j.Creditors {
		line := fmt.Sprintf("    %-40v  %.2f", creditor, j.Credits[i])
		lines = append(lines, line)
	}
	for _, note := range j.Notes {
		lines = append(lines, fmt.Sprintf("    ; %v", note))
	}
	return lines
}

var npayments int

type Payment struct {
	Date      time.Time
	Payee     string
	Debtors   []string
	Creditors []string
	Debits    []float64
	Credits   []float64
	Notes     []string
	parsed    bool
}

func NewPayment(fields ...parsec.ParsecNode) (p *Payment) {
	npayments++

	if fields[4].(string) != "Pymt" {
		panic("impossible situation")
	}

	debtors, debits := []string{}, []float64{}
	creditors, credits := []string{}, []float64{}
	// gather fields
	date, debtor, debit, err := parsefirstblock(fields, 5)
	if err != nil {
		log.Errorf("%v pymt %v", npayments, err)
	}
	debtors, debits = append(debtors, debtor), append(debits, -debit)

	// partition
	offsets, err := dcpartition(fields)
	if err != nil {
		log.Errorf("%v dcpartition: %v\n", npayments, err)
		return nil
	}
	//fmt.Println(npayments, offsets)
	for _, off := range offsets {
		name, amount, err := parseposting(fields[off:])
		if err != nil {
			log.Errorf("%v payment parse debit posting %v\n", npayments, err)
			return nil
		}
		if amount < 0 {
			debtors = append(debtors, name)
			debits = append(debits, -amount)
		} else {
			creditors = append(creditors, name)
			credits = append(credits, -amount)
		}
	}
	// gather notes
	notes := getfnotes(fields, offsets)

	p = &Payment{
		Date: date, Debtors: debtors, Creditors: creditors,
		Debits: debits, Credits: credits, Notes: notes,
		parsed: true,
	}
	return p
}

func (p *Payment) Type() string {
	return "Payment"
}

func (p *Payment) Rewrite(rules map[string]interface{}) {
	p.Debtors = rewritedebtors(rules, p.Debtors)
	p.Creditors = rewritecreditors(rules, p.Creditors)
	p.Payee = rewritepayee(rules, p.Payee)
}

func (p *Payment) ToLedger() []string {
	tm := p.Date.Format("2006-01-02")
	lines := []string{
		fmt.Sprintf("%v  %v; Payment", tm, "Payee"),
	}
	for i, debtor := range p.Debtors {
		line := fmt.Sprintf("    %-40v  %.2f", debtor, p.Debits[i])
		lines = append(lines, line)
	}
	for i, creditor := range p.Creditors {
		line := fmt.Sprintf("    %-40v  %.2f", creditor, p.Credits[i])
		lines = append(lines, line)
	}
	for _, note := range p.Notes {
		lines = append(lines, fmt.Sprintf("    ; %v", note))
	}
	return lines
}

var ncontras int

type Contra struct {
	Date      time.Time
	Payee     string
	Debtors   []string
	Creditors []string
	Debits    []float64
	Credits   []float64
	Notes     []string
	parsed    bool
}

func NewContra(fields ...parsec.ParsecNode) (c *Contra) {
	ncontras++

	if fields[4].(string) != "Ctra" {
		panic("impossible situation")
	}

	debtors, debits := []string{}, []float64{}
	creditors, credits := []string{}, []float64{}
	// gather fields
	date, creditor, credit, err := parsefirstblock(fields, 6)
	if err != nil {
		log.Errorf("%v ctra %v", ncontras, err)
	}
	creditors, credits = append(creditors, creditor), append(credits, -credit)

	// partition
	offsets, err := dcpartition(fields)
	if err != nil {
		log.Errorf("%v dcpartition: %v\n", ncontras, err)
		return nil
	}
	for _, off := range offsets {
		name, amount, err := parseposting(fields[off:])
		if err != nil {
			log.Errorf("%v contra parse debit posting %v\n", ncontras, err)
			return nil
		}
		if amount < 0 {
			debtors = append(debtors, name)
			debits = append(debits, -amount)
		} else {
			creditors = append(creditors, name)
			credits = append(credits, -amount)
		}
	}
	// gather notes
	notes := getfnotes(fields, offsets)

	c = &Contra{
		Date: date, Debtors: debtors, Creditors: creditors,
		Debits: debits, Credits: credits, Notes: notes,
		parsed: true,
	}
	return c
}

func (c *Contra) Type() string {
	return "Contra"
}

func (c *Contra) Rewrite(rules map[string]interface{}) {
	c.Debtors = rewritedebtors(rules, c.Debtors)
	c.Creditors = rewritecreditors(rules, c.Creditors)
	c.Payee = rewritepayee(rules, c.Payee)
}

func (c *Contra) ToLedger() []string {
	tm := c.Date.Format("2006-01-02")
	lines := []string{
		fmt.Sprintf("%v  %v; Contra", tm, "Payee"),
	}
	for i, debtor := range c.Debtors {
		line := fmt.Sprintf("    %-40v  %.2f", debtor, c.Debits[i])
		lines = append(lines, line)
	}
	for i, creditor := range c.Creditors {
		line := fmt.Sprintf("    %-40v  %.2f", creditor, c.Credits[i])
		lines = append(lines, line)
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
		}
	}
	return notes
}

func getfnotes(fields []parsec.ParsecNode, offsets []int) []string {
	offset := len(offsets)
	notes := []string{}
	for _, field := range fields[offset+8:] {
		if s, ok := field.(string); ok {
			if s != "" && strings.HasPrefix(s, "(No. ") == false {
				notes = append(notes, s)
			}
		}
	}
	return notes
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

func warncontent(fields []parsec.ParsecNode) {
	for _, field := range fields {
		if s, ok := field.(string); ok && strings.Trim(s, " ") == "" {
			continue
		} else if _, ok = field.(parsec.MaybeNone); ok {
			continue
		}
		log.Warnf("unknown field %v\n", field)
	}
}

func dcpartition(fields []parsec.ParsecNode) ([]int, error) {
	offsets := []int{}
	for idx := 7; idx+8 < len(fields); {
		if ischequeddcash(fields[idx]) {
			idx += 4
			continue
		}
		offsets = append(offsets, idx)
		idx += 8
	}
	return offsets, nil
}

func ischequeddcash(field parsec.ParsecNode) bool {
	markers := []string{
		"Cash", "Cheque", "Cheque/DD", "Electronic DD/PO",
		"Inter Bank Transfer",
	}
	if s, ok := field.(string); ok {
		token := strings.Trim(s, " ")
		for _, marker := range markers {
			if strings.Contains(token, marker) {
				return true
			}
		}
	}
	return false
}

func parsefirstblock(
	fields []parsec.ParsecNode, nidx int) (time.Time, string, float64, error) {

	date := parsetime(fields[0])
	name := strings.Trim(fields[1].(string), " ")
	if name == "" {
		return date, "", 0, fmt.Errorf("name is empty")
	}
	amount, err := parsefloat(fields, nidx)
	if err != nil {
		return date, "", 0, fmt.Errorf("parsefloat: %v", err)
	}
	return date, name, amount, nil
}

func parseposting(fields []parsec.ParsecNode) (string, float64, error) {
	name := strings.Trim(fields[1].(string), " ")
	if name == "" {
		return "", 0, fmt.Errorf("name is empty")
	}
	amount, err := parsefloat(fields, -1)
	if err != nil {
		return "", 0, fmt.Errorf("parsefloat: %v", err)
	}
	return name, amount, nil
}

func parsefloat(fields []parsec.ParsecNode, n int) (float64, error) {
	if n < 0 {
		for _, field := range fields[6:] {
			if _, ok := field.(parsec.MaybeNone); ok {
				continue
			}
			s, ok := field.(string)
			if ok == false {
				s = field.(*parsec.Terminal).Value
			}
			if s == "" {
				continue
			}
			amount, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return 0, err
			}
			return amount, nil
		}
		return 0, fmt.Errorf("cannot detect amount in fields %v", fields)
	}
	t := fields[n].(*parsec.Terminal)
	amount, err := strconv.ParseFloat(string(t.Value), 64)
	if err != nil {
		return 0, err
	}
	return amount, nil
}

func rewritecreditors(rules map[string]interface{}, creditors []string) []string {
	rc := []string{}
outer:
	for _, creditor := range creditors {
		for from, to := range rules["accountname"].(map[string]interface{}) {
			if creditor == from {
				switch v := to.(type) {
				case string:
					rc = append(rc, v)
				case map[string]interface{}:
					rc = append(rc, v["cr"].(string))
				}
				continue outer
			}
		}
		rc = append(rc, creditor)
	}
	return rc
}

func rewritedebtors(rules map[string]interface{}, debtors []string) []string {
	rc := []string{}
outer:
	for _, debtor := range debtors {
		for from, to := range rules["accountname"].(map[string]interface{}) {
			if debtor == from {
				switch v := to.(type) {
				case string:
					rc = append(rc, v)
				case map[string]interface{}:
					rc = append(rc, v["dr"].(string))
				}
				continue outer
			}
		}
		rc = append(rc, debtor)
	}
	return rc
}

func rewritepayee(rules map[string]interface{}, payee string) string {
	for from, to := range rules["payee"].(map[string]interface{}) {
		if payee == from {
			return to.(string)
		}
	}
	return payee
}
