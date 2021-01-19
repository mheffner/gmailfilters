package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/gmail/v1"
)

// filterfile defines a set of filter objects.
type filterfile struct {
	Filter []filter
}

// filter defines a filter object.
type filter struct {
	Criteria filterCriteria
	Action   filterAction
}

type filterCriteria struct {
	ExcludeChats   bool   `toml:",omitempty"`
	From           string `toml:",omitempty"`
	HasAttachment  bool   `toml:",omitempty"`
	NegatedQuery   string `toml:",omitempty"`
	Query          string `toml:",omitempty"`
	Size           int64  `toml:",omitzero"`
	SizeComparison string `toml:",omitempty"`
	Subject        string `toml:",omitempty"`
	To             string `toml:",omitempty"`
}

type filterAction struct {
	Label   string
	Forward string `toml:",omitempty"`

	Archive  bool `toml:",omitempty"`
	MarkRead bool `toml:",omitempty"`
	Delete   bool `toml:",omitempty"`
}

func (f filter) Compare(o filter) int {
	return f.Criteria.Compare(o.Criteria)
}

// Attempt to check against unset filterCriteria
func (c filterCriteria) isValid() bool {
	if c.Query == "" && c.NegatedQuery == "" && c.From == "" && c.Subject == "" && c.To == "" {
		return false
	}

	return true
}

// this is not an exhaustive compare, but for everything else we'll use a stable sort
func (c filterCriteria) Compare(o filterCriteria) int {
	if ret := strings.Compare(c.To, o.To); ret != 0 {
		return ret
	}
	if ret := strings.Compare(c.From, o.From); ret != 0 {
		return ret
	}
	if ret := strings.Compare(c.Subject, o.Subject); ret != 0 {
		return ret
	}
	if ret := strings.Compare(c.Query, o.Query); ret != 0 {
		return ret
	}
	if ret := strings.Compare(c.NegatedQuery, o.NegatedQuery); ret != 0 {
		return ret
	}

	return 0
}

func (a filterAction) isValid() bool {
	if a.Label == "" && a.Forward == "" && !a.Archive && !a.MarkRead && !a.Delete {
		return false
	}

	return true
}

func (f filter) IsValid() bool {
	return f.Criteria.isValid() && f.Action.isValid()
}

func (f filter) toGmailFilters(labels *labelMap) ([]gmail.Filter, error) {
	if !f.Criteria.isValid() {
		return nil, fmt.Errorf("filter criteria is invalid: %+v", f.Criteria)
	}

	if !f.Action.isValid() {
		return nil, fmt.Errorf("filter action is invalid: %+v", f.Action)
	}

	action := gmail.FilterAction{
		AddLabelIds:    []string{},
		RemoveLabelIds: []string{},
	}
	if len(f.Action.Label) > 0 {
		// Create the label if it does not exist.
		labelID, err := labels.createLabelIfDoesNotExist(f.Action.Label)
		if err != nil {
			return nil, err
		}
		action.AddLabelIds = append(action.AddLabelIds, labelID)
	}

	action.RemoveLabelIds = []string{}
	if f.Action.Archive {
		action.RemoveLabelIds = append(action.RemoveLabelIds, "INBOX")
	}

	if f.Action.MarkRead {
		action.RemoveLabelIds = append(action.RemoveLabelIds, "UNREAD")
	}

	if f.Action.Delete {
		action.AddLabelIds = append(action.AddLabelIds, "TRASH")
	}

	if f.Action.Forward != "" {
		action.Forward = f.Action.Forward
	}

	criteria := gmail.FilterCriteria{
		ExcludeChats:   f.Criteria.ExcludeChats,
		From:           f.Criteria.From,
		HasAttachment:  f.Criteria.HasAttachment,
		NegatedQuery:   f.Criteria.NegatedQuery,
		Query:          f.Criteria.Query,
		Size:           f.Criteria.Size,
		SizeComparison: f.Criteria.SizeComparison,
		Subject:        f.Criteria.Subject,
		To:             f.Criteria.To,
	}

	filter := gmail.Filter{
		Action:   &action,
		Criteria: &criteria,
	}
	filters := []gmail.Filter{
		filter,
	}

	return filters, nil
}

func convertToGmailFilters(filters []filter, labels *labelMap) ([]gmail.Filter, error) {
	gmailFilters := make([]gmail.Filter, 0)
	for _, f := range filters {
		fs, err := f.toGmailFilters(labels)
		if err != nil {
			return nil, err
		}
		gmailFilters = append(gmailFilters, fs...)
	}
	return gmailFilters, nil
}

func addFilters(gmailFilters []gmail.Filter) error {
	// Add the filters.
	for _, fltr := range gmailFilters {
		logrus.WithFields(logrus.Fields{
			"action":   fmt.Sprintf("%#v", fltr.Action),
			"criteria": fmt.Sprintf("%#v", fltr.Criteria),
		}).Debug("adding Gmail filter")
		if _, err := api.Users.Settings.Filters.Create(gmailUser, &fltr).Do(); err != nil {
			return fmt.Errorf("creating filter [%#v] failed: %v", fltr, err)
		}
	}

	return nil
}

func decodeFiltersFile(file string) ([]filter, error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("reading filter file %s failed: %v", file, err)
	}

	var ff filterfile
	meta, err := toml.Decode(string(b), &ff)
	if err != nil {
		return nil, fmt.Errorf("decoding toml failed: %v", err)
	}

	if len(meta.Undecoded()) > 0 {
		return nil, fmt.Errorf("undecoded fields found: %v", meta.Undecoded())
	}

	for _, f := range ff.Filter {
		if !f.IsValid() {
			return nil, fmt.Errorf("filter is invalid: %+v", f)
		}
	}

	return ff.Filter, nil
}

func deleteExistingFilters() error {
	// Get current filters for the user.
	l, err := api.Users.Settings.Filters.List(gmailUser).Do()
	if err != nil {
		return fmt.Errorf("listing filters failed: %v", err)
	}

	// Iterate over the filters.
	for _, f := range l.Filter {
		// Delete the filter.
		if err := api.Users.Settings.Filters.Delete(gmailUser, f.Id).Do(); err != nil {
			return fmt.Errorf("deleting filter id %s failed: %v", f.Id, err)
		}
	}

	return nil
}

func getExistingFilters(labels *labelMap) ([]filter, error) {
	gmailFilters, err := api.Users.Settings.Filters.List(gmailUser).Do()
	if err != nil {
		return nil, err
	}

	var filters []filter

	for _, gmailFilter := range gmailFilters.Filter {
		var f filter
		f.Criteria.ExcludeChats = gmailFilter.Criteria.ExcludeChats
		f.Criteria.From = gmailFilter.Criteria.From
		f.Criteria.HasAttachment = gmailFilter.Criteria.HasAttachment
		f.Criteria.NegatedQuery = gmailFilter.Criteria.NegatedQuery
		f.Criteria.Query = gmailFilter.Criteria.Query
		f.Criteria.Size = gmailFilter.Criteria.Size
		f.Criteria.SizeComparison = gmailFilter.Criteria.SizeComparison
		f.Criteria.Subject = gmailFilter.Criteria.Subject
		f.Criteria.To = gmailFilter.Criteria.To

		if len(gmailFilter.Action.AddLabelIds) > 1 {
			return nil, fmt.Errorf("unable to handle multiple AddLabelIds: %s", gmailFilter.Action.AddLabelIds)
		}

		if len(gmailFilter.Action.AddLabelIds) > 0 {
			labelID := gmailFilter.Action.AddLabelIds[0]
			if labelID == "TRASH" {
				f.Action.Delete = true
			} else {
				l := labels.GetByID(labelID)
				if l == nil {
					return nil, fmt.Errorf("unable to find label with the ID: %s", labelID)
				}
				f.Action.Label = l.Name
			}
		}

		for _, labelID := range gmailFilter.Action.RemoveLabelIds {
			if labelID == "UNREAD" {
				f.Action.MarkRead = true
			} else if labelID == "INBOX" {
				f.Action.Archive = true
			}
		}

		if !f.IsValid() {
			return nil, fmt.Errorf("imported filter is invalid: %+v", f)
		}

		filters = append(filters, f)
	}

	return filters, nil
}

func writeFiltersToFile(filters []filter, file string) error {
	exportFile, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("error exporting filters: %v", err)
	}

	writer := bufio.NewWriter(exportFile)
	encoder := toml.NewEncoder(writer)
	encoder.Indent = ""

	ff := filterfile{
		Filter: filters,
	}

	sort.SliceStable(ff.Filter, func(i, j int) bool {
		return ff.Filter[i].Compare(ff.Filter[j]) < 0
	})

	if err := encoder.Encode(ff); err != nil {
		return fmt.Errorf("error writing file: %v", err)
	}

	fmt.Printf("Exported %d filters\n", len(ff.Filter))

	return nil
}
