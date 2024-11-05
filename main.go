package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type InEvent struct {
	ErrorQuery  string  `yaml:"errorQuery"`
	TotalQuery  string  `yaml:"totalQuery"`
	Coefficient float64 `yaml:"coefficient"`
	FromSLIByID string  `yaml:"fromSLIById"`
}

type InSLI struct {
	Events []InEvent `yaml:"events"`
}

type InSLO struct {
	Name        string   `yaml:"name"`
	Objective   float64  `yaml:"objective"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
	ID          string   `yaml:"id"`
	NotSLO      bool     `yaml:"notSLO"`
	SLI         InSLI    `yaml:"sli"`
}

type InConfig struct {
	SLOS []InSLO `yaml:"slos"`
}

type OutMetadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type OutConfig struct {
	APIVersion string      `yaml:"apiVersion"`
	Kind       string      `yaml:"kind"`
	Metadata   OutMetadata `yaml:"metadata"`
	Spec       OutSpec     `yaml:"spec"`
}

type OutSpec struct {
	Labels  OutLabels `yaml:"labels"`
	Service string    `yaml:"service"`
	SLOs    []OutSLO  `yaml:"slos"`
}

type OutLabels struct {
	Owner string `yaml:"owner"`
	Repo  string `yaml:"repo"`
	Tier  string `yaml:"tier"`
}

type OutSLO struct {
	Name        string      `yaml:"name"`
	Alerting    OutAlerting `yaml:"alerting"`
	Description string      `yaml:"description"`
	Objective   float64     `yaml:"objective"`
	SLI         OutSLI      `yaml:"sli"`
}

type OutAlerting struct {
	Name        string         `yaml:"name"`
	PageAlert   OutAlertConfig `yaml:"pageAlert"`
	TicketAlert OutAlertConfig `yaml:"ticketAlert"`
}

type OutAlertConfig struct {
	Disable bool `yaml:"disable"`
}

type OutSLI struct {
	Events OutEvents `yaml:"events"`
}

type OutEvents struct {
	ErrorQuery string `yaml:"errorQuery"`
	TotalQuery string `yaml:"totalQuery"`
}

const promOne = "1+sum(rate(sloth_slo_info[{{.window}}]))"
const promZeroIfNoData = "or vector(0)"

func buildSLIRecr(slo *InSLO, byIDMap map[string]InSLO, lvl int) string {
	if lvl > 100 {
		panic(fmt.Sprintf("%s max depth cannot be more than 100, check cycle in sli graph!", slo.Name))
	}
	var errs []string
	var sum float64 = 0
	for _, tezEv := range slo.SLI.Events {
		if len(tezEv.FromSLIByID) > 0 {
			innerSLO, ok := byIDMap[tezEv.FromSLIByID]
			if !ok {
				panic(fmt.Sprintf("%s - unknown fromSLIByID", tezEv.FromSLIByID))
			}
			innerSLIErr := buildSLIRecr(&innerSLO, byIDMap, lvl+1)
			errs = append(errs, fmt.Sprintf("(%f * (%s))", tezEv.Coefficient, innerSLIErr))
		} else {
			errs = append(errs, fmt.Sprintf("(%f * (((%s)/(%s)) %s))", tezEv.Coefficient, tezEv.ErrorQuery, tezEv.TotalQuery, promZeroIfNoData))
		}
		sum += tezEv.Coefficient
	}

	if math.Abs(sum-1) > 10e-2 {
		panic(fmt.Sprintf("%s sum of coefficient's != 1", slo.Name))
	}

	return fmt.Sprintf("%s", strings.Join(errs, " + "))
}

func main() {
	var (
		inFile        = ""
		outFilePrefix = ""
		repo          = "https://github.com/some/service"
		tier          = "1"
		owner         = "auth"
		service       = "auth-login"
		useNotSlo     = false
	)
	flag.StringVar(&inFile, "in", "./slo.yml", "-in=./slo.yml")
	flag.StringVar(&outFilePrefix, "outp", "./slo/out", "-outp=./slo/out")
	flag.StringVar(&repo, "repo", "https://github.com/some/service", "-repo=https://github.com/some/service")
	flag.StringVar(&tier, "tier", "1", "-tier=1")
	flag.StringVar(&owner, "owner", "auth", "-owner=auth")
	flag.StringVar(&service, "service", "auth-login", "-service=auth-login")
	flag.BoolVar(&useNotSlo, "usenotslo", false, "-usenotslo=true")
	flag.Parse()

	inFile, _ = filepath.Abs(inFile)
	err := os.MkdirAll(outFilePrefix, os.ModePerm)
	if err != nil {
		panic(err)
	}

	file, err := os.Open(inFile)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	var config InConfig
	if err := yaml.NewDecoder(file).Decode(&config); err != nil {
		panic(err)
	}

	var byIDMap = map[string]InSLO{}
	for _, slo := range config.SLOS {
		byIDMap[slo.ID] = slo
	}

	for i, slo := range config.SLOS {
		if useNotSlo && slo.NotSLO {
			continue
		}
		totalErrs := buildSLIRecr(&slo, byIDMap, 1)

		outFile := outFilePrefix + fmt.Sprintf("sloth_%s.yml", slo.Name)
		fmt.Printf("generated: %s\n", outFile)
		outputFile, err := os.Create(outFile)
		if err != nil {
			panic(err)
		}

		if err := yaml.NewEncoder(outputFile).Encode(&OutConfig{
			APIVersion: "sloth.slok.dev/v1",
			Kind:       "PrometheusServiceLevel",
			Metadata: OutMetadata{
				Name:      fmt.Sprintf("sloth-slo-%s-%d", service, i),
				Namespace: service,
			},
			Spec: OutSpec{
				Service: service,
				Labels: OutLabels{
					Tier:  tier,
					Repo:  repo,
					Owner: owner,
				},
				SLOs: []OutSLO{
					{
						Name:        slo.Name,
						Objective:   slo.Objective,
						Description: slo.Description,
						Alerting: OutAlerting{
							Name:        slo.Name + "_alert",
							PageAlert:   OutAlertConfig{Disable: true},
							TicketAlert: OutAlertConfig{Disable: true},
						},
						SLI: OutSLI{
							Events: OutEvents{
								ErrorQuery: totalErrs,
								TotalQuery: promOne,
							},
						},
					},
				},
			}}); err != nil {
			panic(err)
		}

		outputFile.Close()
	}

}
