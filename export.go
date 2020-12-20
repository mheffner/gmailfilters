package main

import "fmt"

func exportExisting(labelMap *labelMap, filtersFile string, labelsFile string) error {
	fmt.Print("exporting existing filters and labels...\n")

	filters, err := getExistingFilters(labelMap)
	if err != nil {
		return fmt.Errorf("error downloading existing filters: %v", err)
	}

	if err := writeFiltersToFile(filters, filtersFile); err != nil {
		return err
	}

	if err := writeLabelsToFile(labelMap, labelsFile); err != nil {
		return err
	}

	return nil
}
