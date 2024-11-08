# slox

[Sloth](https://sloth.dev/) generator for weighted hierarchical SLI/SLO

SLI/SLI is hierarchically based on domain knowledge about the service/business.

This generator allows you to generate a weighted SLI based on other SLIS,
possibly also weighted ones - for the SLOTH graph plugin

# Example

Let's define a set of our SLIS for each of the entities we want to monitor, at each level of detail

We get a vector for a certain level: `sli = [sli_1 , ... , sli_N]`

Based on our domain knowledge, we will determine the importance of each of the SLI components in the form of weights, the sum of which = 1

We get the vector of weights: `w_c = [w_c1, ... ,w_cN]`

The resulting final sli for the parent level will be calculated as a linear combination `<w_c, sli>`

# Example File
```yaml
slos:
  - name: "main_sli" # - SLO Name
    objective: 99.9 # - SLO Objective
    description: "main_sli"
    id: "main_sli"
    sli:
      events:
        - fromSLIById: first_sli
          coefficient: 0.50
        - fromSLIById: third_sli
          coefficient: 0.45

  - name: "first_sli"
    objective: 99.9
    description: "first_sli"
    id: "first_sli"
    sli:
      events:
        - fromSLIById: second_sli # Metric from other recursive SLI 
          coefficient: 0.3
        - errorQuery: sum(rate(some_metrics{code=~"5.*"}[{{.window}}])) # Metric that counts bad events with the window {{.window}} - templating (for different windows)
          totalQuery: sum(rate(some_metrics{}[{{.window}}])) # Metric that counts total events with the window {{.window}} - templating (for different windows)
          coefficient: 0.7 # - Coefficient (w_ci from the model description) with which this SLI will be weighted (i.e., the coefficient with which we include this part of the SLI in the overall SLI, for example = 0.5 * err1-1 + 0.5 * err1-2)

  - name: "second_sli"
    objective: 99.9
    description: "second_sli"
    id: "second_sli"
    notSLO: true # - Is this NOT an SLO? (used when it's a fake SLO just to combine SLIs without creating an SLO target in Grafana)
    sli:
      events:
        - errorQuery: sum(rate(some_metrics_second{code=~"5.*"}[{{.window}}]))
          totalQuery: sum(rate(some_metrics_second{}[{{.window}}]))
          coefficient: 1.0
          
  - name: "third_sli"
    objective: 99.9
    description: "third_sli"
    id: "third_sli"
    notSLO: true
    sli:
        events:
          - errorQuery: sum(rate(some_metrics_third_v1{code=~"5.*"}[{{.window}}]))
            totalQuery: sum(rate(some_metrics_third_v1{}[{{.window}}]))
            coefficient: 0.30
          - errorQuery: sum(rate(some_metrics_third_v2{code=~"5.*"}[{{.window}}]))
            totalQuery: sum(rate(some_metrics_third_v2{}[{{.window}}]))
            coefficient: 0.7
```
# Example generate

```shell
$ slox -in=./example/slox.yaml -outp=./example/generated/ -repo=https://github.com/some/service -tier=1 -owner=auth -service=auth-login -usenotslo=true
```

# Example output
Check path -> `generated/*.yml`