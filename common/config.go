package common

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ProcessEmails []EmailProcessingConfig `yaml:"process_emails"`
}

type EmailProcessingConfig struct {
	FromEmail       string           `yaml:"fromEmail"`
	ProcessingSteps []ProcessingStep `yaml:"processingSteps"`
}

type ProcessingStep struct {
	OptionName      string           `yaml:"optionName"`
	Discriminator   Discriminator    `yaml:"discriminator"`
	SourceAccountId int              `yaml:"sourceAccountId"`
	ExtractionSteps []ExtractionStep `yaml:"extractionSteps"`
}

type Discriminator struct {
	Type  string `yaml:"type"`
	Regex string `yaml:"regex"`
}

type ExtractionStep struct {
	Type         string        `yaml:"type"`
	Regex        string        `yaml:"regex"`
	TargetFields []TargetField `yaml:"targetFields"`
}

type TargetField struct {
	GroupNumber int    `yaml:"groupNumber"`
	TargetField string `yaml:"targetField"`
}

func GetConfig(configFileLocation string) (*Config, error) {
	data, err := ioutil.ReadFile(configFileLocation)
	if err != nil {
		return nil, err
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
