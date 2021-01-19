package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/gmail/v1"
)

type labelFile struct {
	Label []*label
}

type label struct {
	Id              string
	Name            string
	BackgroundColor string `toml:",omitempty"`
	TextColor       string `toml:",omitempty"`

	// LabelListVisbility: hide, show, showifunread
	LabelListVisibility string `toml:",omitempty"`

	// MessageListVisbility: hide or show
	MessageListVisibility string `toml:",omitempty"`

	// system or user
	Type string
}

type labelMap struct {
	byName map[string]*label
	byID   map[string]*label
}

// Compare is used for sorting and we sort on name only
func (l *label) Compare(o *label) int {
	return strings.Compare(l.Name, o.Name)
}

// Equals returns a deep comparison, fixing differences in default values
func (l *label) Equals(o *label) bool {
	fixed := *o

	if l.MessageListVisibility == "show" && fixed.MessageListVisibility == "" {
		fixed.MessageListVisibility = "show"
	}

	if l.LabelListVisibility == "show" && fixed.LabelListVisibility == "" {
		fixed.LabelListVisibility = "show"
	}

	return *l == fixed
}

func (l *label) IsValid() bool {
	if l.Id == "" || l.Name == "" {
		return false
	}

	if l.LabelListVisibility != "" && l.LabelListVisibility != "hide" &&
		l.LabelListVisibility != "show" && l.LabelListVisibility != "showifunread" {
		return false
	}

	if l.MessageListVisibility != "" && l.MessageListVisibility != "hide" && l.MessageListVisibility != "show" {
		return false
	}

	if l.Type != "user" && l.Type != "system" {
		return false
	}

	// Both colors have to be specified
	if (l.BackgroundColor != "" && l.TextColor == "") || (l.TextColor != "" && l.BackgroundColor == "") {
		return false
	}

	return true
}

func (l *label) UpdateLabel() error {
	updateLabel := &gmail.Label{
		Id:                    l.Id,
		MessageListVisibility: l.MessageListVisibility,
		Name:                  l.Name,
		Type:                  l.Type,
		NullFields:            []string{},
	}

	if l.BackgroundColor != "" || l.TextColor != "" {
		updateLabel.Color = &gmail.LabelColor{
			BackgroundColor: l.BackgroundColor,
			TextColor:       l.TextColor,
		}
	} else {
		updateLabel.NullFields = append(updateLabel.NullFields, "Color")
	}

	switch l.LabelListVisibility {
	case "hide":
		updateLabel.LabelListVisibility = "labelHide"
	case "show":
		updateLabel.LabelListVisibility = "labelShow"
	case "showifunread":
		updateLabel.LabelListVisibility = "labelShowIfUnread"
	}

	fmt.Printf("Updating label: %s\n", updateLabel.Name)
	_, err := api.Users.Labels.Patch(gmailUser, l.Id, updateLabel).Do()
	if err != nil {
		return fmt.Errorf("failed to update label %s: %v", l.Name, err)
	}

	return nil
}

func getLabelMap() (*labelMap, error) {
	// Get the labels for the user and map its name to its ID.
	labelList, err := api.Users.Labels.List(gmailUser).Do()
	if err != nil {
		return nil, fmt.Errorf("listing labels failed: %v", err)
	}

	labels := labelMap{
		byName: make(map[string]*label, len(labelList.Labels)),
		byID:   make(map[string]*label, len(labelList.Labels)),
	}
	for _, lbl := range labelList.Labels {
		labels.Add(labelFromAPI(lbl))
	}

	return &labels, nil
}

func (m *labelMap) Add(lbl *label) {
	m.byName[strings.ToLower(lbl.Name)] = lbl
	m.byID[lbl.Id] = lbl
}

func (m *labelMap) GetByName(name string) *label {
	return m.byName[strings.ToLower(name)]
}

func (m *labelMap) GetByID(id string) *label {
	return m.byID[id]
}

func (m *labelMap) createLabelIfDoesNotExist(name string) (string, error) {
	// Try to find the label.
	l := m.GetByName(name)
	if l != nil {
		// We found the label.
		return l.Id, nil
	}

	// Create the label if it does not exist.
	lbl, err := api.Users.Labels.Create(gmailUser, &gmail.Label{Name: name}).Do()
	if err != nil {
		return "", fmt.Errorf("creating label %s failed: %v", name, err)
	}
	logrus.Infof("Created label: %s", name)

	m.Add(labelFromAPI(lbl))
	return lbl.Id, nil
}

func labelFromAPI(lbl *gmail.Label) *label {
	var l label

	l.Name = lbl.Name
	l.Id = lbl.Id
	if lbl.Color != nil {
		l.BackgroundColor = lbl.Color.BackgroundColor
		l.TextColor = lbl.Color.TextColor
	}

	l.LabelListVisibility = strings.TrimPrefix(strings.ToLower(lbl.LabelListVisibility), "label")
	l.MessageListVisibility = strings.ToLower(lbl.MessageListVisibility)

	l.Type = lbl.Type

	return &l
}

func writeLabelsToFile(labelMap *labelMap, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error exporting labels: %v", err)
	}

	writer := bufio.NewWriter(file)
	encoder := toml.NewEncoder(writer)
	encoder.Indent = ""

	lf := &labelFile{
		Label: make([]*label, 0, len(labelMap.byName)),
	}

	// build sorted file
	for _, v := range labelMap.byName {
		lf.Label = append(lf.Label, v)
	}

	sort.SliceStable(lf.Label, func(i, j int) bool {
		return lf.Label[i].Compare(lf.Label[j]) < 0
	})

	if err := encoder.Encode(lf); err != nil {
		return fmt.Errorf("error writing file: %v, labels: %+v", err, lf)
	}

	fmt.Printf("Exported %d labels\n", len(lf.Label))

	return nil
}

func decodeLabelsFile(file string) ([]*label, error) {
	var lf labelFile
	meta, err := toml.DecodeFile(file, &lf)
	if err != nil {
		return nil, fmt.Errorf("decoding labels toml failed: %v", err)
	}

	if len(meta.Undecoded()) > 0 {
		return nil, fmt.Errorf("undecoded fields found: %v", meta.Undecoded())
	}

	for _, l := range lf.Label {
		if !l.IsValid() {
			return nil, fmt.Errorf("label %s is invalid", l.Name)
		}
	}

	return lf.Label, nil
}

func updateLabels(labels *labelMap, localLabels []*label) error {
	updated := 0
	for _, l := range localLabels {
		rl := labels.GetByID(l.Id)
		if rl == nil {
			fmt.Printf("Local label %s not found remotely, will remove\n", l.Name)
			continue
		}

		if !rl.Equals(l) {
			*rl = *l
			if err := rl.UpdateLabel(); err != nil {
				return err
			}
			updated++
		}
	}

	if updated > 0 {
		fmt.Printf("updated %d labels\n", updated)
	}

	return nil
}
