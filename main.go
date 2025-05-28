package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv" // For converting string to int for ease options

	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
)

// AnkiConnect API endpoint
const ankiConnectURL = "http://localhost:8765"

// Card represents a simplified Anki card for our TUI
type Card struct {
	ID    int64
	Front string
	Back  string
	EaseOptions map[int]string // Map ease value (1-4) to button text (e.g., "Again", "Good")
}

// AnkiConnectRequest is a generic structure for AnkiConnect API calls
type AnkiConnectRequest struct {
	Action  string      `json:"action"`
	Version int         `json:"version"`
	Params  interface{} `json:"params"`
}

// AnkiConnectResponse is a generic structure for AnkiConnect API responses
type AnkiConnectResponse struct {
	Result interface{} `json:"result"`
	Error  interface{} `json:"error"` // AnkiConnect uses null for no error, otherwise a string
}

// findCardsParams for the findCards action
type findCardsParams struct {
	Query string `json:"query"`
}

// cardsInfoParams for the cardsInfo action
type cardsInfoParams struct {
	Cards []int64 `json:"cards"`
}

// cardInfoResult represents a single card's info from cardsInfo response
type cardInfoResult struct {
	CardID int64 `json:"cardId"`
	Fields struct {
		Front struct {
			Value string `json:"value"`
		} `json:"Front"`
		Back struct {
			Value string `json:"value"`
		} `json:"Back"`
	} `json:"fields"`
	Buttons []int `json:"buttons"` // Array of ease values (1, 2, 3, 4)
}

// answerCardsParams for the answerCards action
type answerCardsParams struct {
	CardID int64 `json:"cardId"`
	Ease   int   `json:"ease"`
}

// Msg types for async operations
type (
	cardsLoadedMsg []Card // Sent when cards are successfully loaded
	errMsg         error  // Sent when an error occurs
	cardAnsweredMsg struct {
		cardID int64
		ease   int
	} // Sent when a card is successfully answered
)

// model represents the state of our TUI application
type model struct {
	cards            []Card
	currentCardIndex int
	showBack         bool
	state            appState
	err              error
	ankiConnectURL   string
	quitting         bool
	styles           styles
}

// appState defines the different states of the application
type appState int

const (
	stateLoading appState = iota
	stateDisplayingCard
	stateNoCards
	stateError
	stateQuitting
)

// styles for lipgloss
type styles struct {
	title  lipgloss.Style
	status lipgloss.Style
	card   lipgloss.Style
	front  lipgloss.Style
	back   lipgloss.Style
	prompt lipgloss.Style
	error  lipgloss.Style
	button lipgloss.Style
}

// newStyles initializes and returns the lipgloss styles
func newStyles() styles {
	return styles{
		title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Padding(0, 1),
		status: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Padding(0, 1),
		card: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder(), true).
			BorderForeground(lipgloss.Color("#6243A6")).
			Padding(1, 2).
			Width(60).
			Align(lipgloss.Center),
		front: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1),
		back: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DDDDDD")).
			Padding(0, 1),
		prompt: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA")).
			PaddingTop(1),
		error: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true).
			Padding(0, 1),
		button: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#5A56E0")).
			Padding(0, 1).
			MarginRight(1),
	}
}

// InitialModel returns the initial state of the model
func InitialModel() model {
	return model{
		state:          stateLoading,
		ankiConnectURL: ankiConnectURL,
		styles:         newStyles(),
	}
}

// Init is called once when the program starts. It performs initial setup.
func (m model) Init() tea.Cmd {
	return m.fetchDueCardsCmd() // Start fetching cards immediately
}

// Update handles messages and updates the model's state
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if m.state == stateDisplayingCard {
				if m.showBack {
					// If back is shown, pressing enter moves to next card (if no ease option selected)
					// Or, if ease options are displayed, it does nothing until an ease button is pressed.
					return m, nil
				}
				m.showBack = true // Flip card to show back
			}

		case "1", "2", "3", "4":
			if m.state == stateDisplayingCard && m.showBack {
				ease, err := strconv.Atoi(msg.String())
				if err != nil {
          m.err = fmt.Errorf("invalid ease input: %w", err) // Set the error directly on the model
					m.state = stateError                               // Change the app state to error
					return m, nil  
        }
				if m.currentCardIndex < len(m.cards) {
					card := m.cards[m.currentCardIndex]
					// Check if the selected ease value is valid for the current card
					if _, ok := card.EaseOptions[ease]; ok {
						return m, m.answerCardCmd(card.ID, ease)
					}
				}
			}

		case "right", "n": // For debugging or skipping cards without answering (not recommended for actual study)
			if m.state == stateDisplayingCard {
				m.currentCardIndex++
				m.showBack = false
				if m.currentCardIndex >= len(m.cards) {
					m.state = stateNoCards // All cards studied/skipped
				}
			}
		}

	case cardsLoadedMsg:
		if len(msg) == 0 {
			m.state = stateNoCards
		} else {
			m.cards = msg
			m.state = stateDisplayingCard
			m.currentCardIndex = 0
			m.showBack = false
		}
		return m, nil

	case cardAnsweredMsg:
		// Move to the next card after answering
		m.currentCardIndex++
		m.showBack = false
		if m.currentCardIndex >= len(m.cards) {
			m.state = stateNoCards // All cards studied
		}
		return m, nil

	case errMsg:
		m.err = msg
		m.state = stateError
		return m, nil
	}

	return m, nil
}

// View renders the TUI
func (m model) View() string {
	if m.quitting {
		return "Exiting Anki Study TUI...\n"
	}

	s := ""
	header := m.styles.title.Render("Anki Study TUI")
	status := ""

	switch m.state {
	case stateLoading:
		s = "Loading cards from AnkiConnect...\n"
	case stateDisplayingCard:
		if len(m.cards) > 0 && m.currentCardIndex < len(m.cards) {
			card := m.cards[m.currentCardIndex]
			status = m.styles.status.Render(fmt.Sprintf("Card %d/%d", m.currentCardIndex+1, len(m.cards)))
			cardContent := m.styles.front.Render(card.Front)
			if m.showBack {
				cardContent += "\n\n" + m.styles.back.Render(card.Back)
				cardContent += "\n\n" + m.styles.prompt.Render("Press 1-4 to answer:")
				for easeVal, easeText := range card.EaseOptions {
					cardContent += fmt.Sprintf(" %s", m.styles.button.Render(fmt.Sprintf("%d: %s", easeVal, easeText)))
				}
			} else {
				cardContent += "\n\n" + m.styles.prompt.Render("Press ENTER to reveal back")
			}
			s = m.styles.card.Render(cardContent)
		}
	case stateNoCards:
		s = "No cards due today! Great job!\n"
		s += m.styles.prompt.Render("Press 'q' to quit.")
	case stateError:
		s = m.styles.error.Render(fmt.Sprintf("Error: %v\n", m.err))
		s += m.styles.prompt.Render("Press 'q' to quit.")
	}

	return lipgloss.JoinVertical(lipgloss.Center, header, status, s)
}

// postAnkiConnect sends a request to the AnkiConnect API
func postAnkiConnect(action string, version int, params interface{}) (interface{}, error) {
	reqBody, err := json.Marshal(AnkiConnectRequest{
		Action:  action,
		Version: version,
		Params:  params,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(ankiConnectURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to AnkiConnect at %s: %w\nEnsure Anki is running and AnkiConnect add-on is installed.", ankiConnectURL, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var ankiResp AnkiConnectResponse
	if err := json.Unmarshal(body, &ankiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AnkiConnect response: %w", err)
	}

	if ankiResp.Error != nil {
		return nil, fmt.Errorf("AnkiConnect error: %v", ankiResp.Error)
	}

	return ankiResp.Result, nil
}

// fetchDueCardsCmd is a tea.Cmd that fetches due cards asynchronously
func (m model) fetchDueCardsCmd() tea.Cmd {
	return func() tea.Msg {
		// 1. Find due card IDs
		findCardsResult, err := postAnkiConnect("findCards", 6, findCardsParams{Query: "is:due"})
		if err != nil {
			return errMsg(fmt.Errorf("failed to find due cards: %w", err))
		}

		cardIDs := []int64{}
		if ids, ok := findCardsResult.([]interface{}); ok {
			for _, id := range ids {
				if floatID, ok := id.(float64); ok { // JSON numbers are often float64 in Go
					cardIDs = append(cardIDs, int64(floatID))
				}
			}
		} else {
			return errMsg(fmt.Errorf("unexpected findCards result format: %T", findCardsResult))
		}

		if len(cardIDs) == 0 {
			return cardsLoadedMsg{} // No cards due
		}

		// 2. Get detailed info for these cards
		cardsInfoResult, err := postAnkiConnect("cardsInfo", 6, cardsInfoParams{Cards: cardIDs})
		if err != nil {
			return errMsg(fmt.Errorf("failed to get cards info: %w", err))
		}

		ankiCards := []Card{}
		if infos, ok := cardsInfoResult.([]interface{}); ok {
			for _, info := range infos {
				infoBytes, err := json.Marshal(info) // Marshal back to bytes to unmarshal into specific struct
				if err != nil {
					log.Printf("Warning: failed to marshal card info for parsing: %v", err)
					continue
				}
				var ci cardInfoResult
				if err := json.Unmarshal(infoBytes, &ci); err != nil {
					log.Printf("Warning: failed to unmarshal card info into struct: %v", err)
					continue
				}

				easeOptions := make(map[int]string)
				// AnkiConnect doesn't directly provide button text, but we can infer them
				// based on common Anki ease values.
				// The 'buttons' array gives the ease values (1,2,3,4) in order.
				// We need to map these to typical Anki button labels.
				// A more robust solution might involve 'getDeckConfig' or 'getNoteTypes'
				// but for simplicity, we'll use standard labels.
				defaultEaseLabels := map[int]string{
					1: "Again",
					2: "Hard",
					3: "Good",
					4: "Easy",
				}
				for _, easeVal := range ci.Buttons {
					if label, ok := defaultEaseLabels[easeVal]; ok {
						easeOptions[easeVal] = label
					} else {
						// Fallback if an unexpected ease value appears
						easeOptions[easeVal] = fmt.Sprintf("Ease %d", easeVal)
					}
				}

				ankiCards = append(ankiCards, Card{
					ID:    ci.CardID,
					Front: ci.Fields.Front.Value,
					Back:  ci.Fields.Back.Value,
					EaseOptions: easeOptions,
				})
			}
		} else {
			return errMsg(fmt.Errorf("unexpected cardsInfo result format: %T", cardsInfoResult))
		}

		return cardsLoadedMsg(ankiCards)
	}
}

// answerCardCmd is a tea.Cmd that answers a card asynchronously
func (m model) answerCardCmd(cardID int64, ease int) tea.Cmd {
	return func() tea.Msg {
		_, err := postAnkiConnect("answerCards", 6, answerCardsParams{
			CardID: cardID,
			Ease:   ease,
		})
		if err != nil {
			return errMsg(fmt.Errorf("failed to answer card %d with ease %d: %w", cardID, ease, err))
		}
		return cardAnsweredMsg{cardID: cardID, ease: ease}
	}
}

func main() {
	p := tea.NewProgram(InitialModel())

	if _, err := p.Run(); err != nil {
		log.Fatalf("Alas, there's been an error: %v", err)
	}
}

