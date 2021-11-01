package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/moncho/warpwallet/bitcoin"
)

const (
	mempool = "https://mempool.space/api/address/%s"
)

type btcAddressData struct {
	Address    string `json:"address"`
	ChainStats struct {
		FundedTxoSum int `json:"funded_txo_sum"`
		SpentTxoSum  int `json:"spent_txo_sum"`
	} `json:"chain_stats"`
}

func (d *btcAddressData) hasTransactions() bool {
	return d.ChainStats.FundedTxoSum > 0
}

func main2() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter wallet passphrase: ")
	text, _ := reader.ReadBytes('\n')
	text = text[:len(text)-1]
	fmt.Printf("%s\n", string(text))
	address, err := newAddress(string(text))
	if err != nil {
		panic(err)
	}
	fmt.Printf("Public Address: %s\n", address)
	balance, err := addressData(address)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Has transactions? %t\n", balance.hasTransactions())
	if balance.hasTransactions() {
		fmt.Printf("Funded balance: %d\n", balance.ChainStats.FundedTxoSum)
		fmt.Printf("Spent balance: %d\n", balance.ChainStats.SpentTxoSum)
	}
}

func main() {
	p := tea.NewProgram(newModel())

	if err := p.Start(); err != nil {
		log.Fatal(err)
	}
}

type errMsg error

type model struct {
	textInput textinput.Model
	err       error
}

func newModel() model {
	ti := textinput.NewModel()
	ti.Placeholder = "brainwallet passphrase"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40

	return model{
		textInput: ti,
	}
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			passphrase := m.textInput.Value()
			address, err := newAddress(passphrase)
			if err != nil {
				m.err = err
				break
			}
			addressData, err := addressData(address)
			if err != nil {
				m.err = err
				break
			}
			return walletModel{
				address:     address,
				passphrase:  passphrase,
				addressData: addressData,
			}, nil

		}
	case errMsg:
		m.err = msg
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return fmt.Sprintf(
		"Enter wallet passphrase:\n\n%s\n\n%s",
		m.textInput.View(),
		"(esc to quit)",
	) + "\n"
}

type walletModel struct {
	passphrase  string
	address     string
	addressData btcAddressData
}

func (m walletModel) Init() tea.Cmd {
	return nil
}

func (m walletModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit
		case tea.KeyEsc:
			return newModel(), nil
		}
	}
	return m, nil
}

var style = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#FAFAFA")).
	PaddingLeft(2)

func (m walletModel) View() string {
	var builder strings.Builder
	builder.WriteString(
		lipgloss.NewStyle().
			Bold(true).
			PaddingLeft(1).
			Foreground(lipgloss.Color("#F39C12")).Render("Address "))
	builder.WriteString(m.address)
	builder.WriteString("\n")
	builder.WriteString(
		style.Render("Has transactions? "))
	builder.WriteString(fmt.Sprintf("%v\n", m.addressData.hasTransactions()))
	if m.addressData.hasTransactions() {
		builder.WriteString(
			style.Render("Funded balance: "))
		builder.WriteString(fmt.Sprintf("%d\n", m.addressData.ChainStats.FundedTxoSum))
		builder.WriteString(
			style.Render("Spent balance: "))
		builder.WriteString(fmt.Sprintf("%d\n", m.addressData.ChainStats.SpentTxoSum))
	}
	builder.WriteString("\n (esc to try another) \n")
	return fmt.Sprintf(builder.String())
}

func newAddress(passphrase string) (string, error) {
	hash1 := sha256.Sum256([]byte(passphrase))
	priv, err := bitcoin.NewBitcoinPrivKey(hash1[:])
	if err != nil {
		return "", err
	}
	return bitcoin.ToBTCAddress(priv.PublicKey)
}

func addressData(address string) (btcAddressData, error) {
	var data btcAddressData
	resp, err := http.Get(fmt.Sprintf(mempool, address))
	if err != nil {
		return data, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return data, fmt.Errorf("mempool.space read body: %s", err)
	}
	if resp.StatusCode != 200 {
		return data, fmt.Errorf("mempool.space http status: %s", string(body))
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		return data, fmt.Errorf("mempool.space response unmarshal: %s", err)
	}
	return data, nil
}
