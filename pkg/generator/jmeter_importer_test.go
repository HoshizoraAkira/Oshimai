package generator

import (
	"context"
	"testing"
)

const sampleJMX = `<?xml version="1.0" encoding="UTF-8"?>
<jmeterTestPlan version="1.2">
  <hashTree>
    <TestPlan testname="Demo Plan"/>
    <hashTree>
      <ThreadGroup testname="Users"/>
      <hashTree>
        <HTTPSamplerProxy testname="Login">
          <stringProp name="HTTPSampler.domain">api.toko.co.id</stringProp>
          <stringProp name="HTTPSampler.port"></stringProp>
          <stringProp name="HTTPSampler.protocol">https</stringProp>
          <stringProp name="HTTPSampler.path">/v1/login</stringProp>
          <stringProp name="HTTPSampler.method">POST</stringProp>
          <elementProp name="HTTPsampler.Arguments" elementType="Arguments">
            <collectionProp name="Arguments.arguments">
              <elementProp name="" elementType="HTTPArgument">
                <stringProp name="Argument.value">{"user":"demo"}</stringProp>
              </elementProp>
            </collectionProp>
          </elementProp>
        </HTTPSamplerProxy>
        <hashTree/>
        <HTTPSamplerProxy testname="List Items">
          <stringProp name="HTTPSampler.domain">api.toko.co.id</stringProp>
          <stringProp name="HTTPSampler.protocol">https</stringProp>
          <stringProp name="HTTPSampler.path">/v1/items</stringProp>
          <stringProp name="HTTPSampler.method">GET</stringProp>
        </HTTPSamplerProxy>
        <hashTree/>
      </hashTree>
    </hashTree>
  </hashTree>
</jmeterTestPlan>`

func TestImportJMeterPlan(t *testing.T) {
	sc, err := ImportJMeterPlan(context.Background(), []byte(sampleJMX), GeneratorConfig{})
	if err != nil {
		t.Fatalf("ImportJMeterPlan failed: %v", err)
	}
	if len(sc.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(sc.Steps))
	}
	if sc.BaseURL != "https://api.toko.co.id" {
		t.Errorf("expected base URL derived from the first sampler, got %s", sc.BaseURL)
	}

	first := sc.Steps[sc.InitialStepID]
	if first.Request.Method != "POST" || first.Request.Path != "/v1/login" {
		t.Errorf("expected first step to be POST /v1/login in document order, got %s %s", first.Request.Method, first.Request.Path)
	}
	if first.Request.Body == "" {
		t.Error("expected the Login sampler's Argument.value to populate the request body")
	}
}

func TestImportJMeterPlanRejectsNoSamplers(t *testing.T) {
	empty := `<jmeterTestPlan><hashTree><TestPlan/></hashTree></jmeterTestPlan>`
	if _, err := ImportJMeterPlan(context.Background(), []byte(empty), GeneratorConfig{}); err == nil {
		t.Error("expected error for a test plan with no HTTP samplers")
	}
}

func TestImportJMeterPlanRejectsMalformedXML(t *testing.T) {
	if _, err := ImportJMeterPlan(context.Background(), []byte("not xml at all <<<"), GeneratorConfig{}); err == nil {
		t.Error("expected error for malformed XML")
	}
}
