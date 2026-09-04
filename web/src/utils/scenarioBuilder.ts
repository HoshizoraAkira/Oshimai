import { FlowNode, Scenario, ScenarioStep } from '../types/models';

/**
 * Builds a valid finite-state machine Scenario object from canvas FlowNode items.
 */
export function buildScenarioObject(nodes: FlowNode[], scenarioId: string, baseUrl: string): Scenario {
  const steps: Record<string, ScenarioStep> = {};
  let initialStepId = nodes[0]?.id || 'step_1';

  nodes.forEach(node => {
    if (node.type === 'note') return;

    if (node.isInitial) {
      initialStepId = node.id;
    }

    const stepObj: ScenarioStep = {
      id: node.id,
      name: node.name,
      transitions: (node.transitions || []).map(t => ({
        target_step_id: t.target_step_id,
        probability: t.probability ?? 1.0,
      })),
    };

    if (node.type === 'http') {
      const headers: Record<string, string> = { ...(node.headers || {}) };
      if (node.authBearer) {
        headers['Authorization'] = `Bearer ${node.authBearer}`;
      }

      stepObj.request = {
        method: node.method || 'GET',
        path: node.path || '/',
        headers: Object.keys(headers).length > 0 ? headers : undefined,
        body: node.body || undefined,
      };

      if (node.assertions && node.assertions.length > 0) {
        stepObj.assertions = node.assertions;
      }
      if (node.extractors && node.extractors.length > 0) {
        stepObj.extractors = node.extractors;
      }
    } else if (node.type === 'delay') {
      // The engine recognizes a pure pause step by think_time with no request.path —
      // delay_ms isn't a field it understands, so sending that instead silently turned
      // every delay node into a live GET request to the base URL. think_time must be a
      // Go duration string ("1500ms") here: the launcher submits this as scenario_yaml,
      // which the backend parses with its YAML (not JSON) decoder, and Duration's
      // UnmarshalYAML only accepts a duration string or falls back to a raw int64 of
      // nanoseconds — a bare unitless number like "1500000000" decodes as a string first
      // and then fails time.ParseDuration for lacking a unit suffix.
      stepObj.think_time = `${node.delayMs || 1000}ms`;
    } else if (node.type === 'decision') {
      // Per-step conditional branching isn't implemented by the engine yet (conditions
      // only exist on individual transitions) — condition is kept for forward
      // compatibility, and a nominal think_time keeps this a harmless pass-through
      // instead of firing an unintended GET request to the base URL.
      stepObj.condition = node.condition || '${status} == 200';
      stepObj.think_time = '1ms';
    }

    steps[node.id] = stepObj;
  });

  return {
    id: scenarioId || 'custom_scenario',
    name: scenarioId || 'Custom Scenario',
    version: '1.0',
    base_url: baseUrl || 'https://httpbin.org',
    initial_step_id: initialStepId,
    steps: steps,
  };
}
