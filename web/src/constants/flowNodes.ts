import { FlowNode } from '../types/models';

export const DEFAULT_NODES: FlowNode[] = [
  {
    id: 'login',
    name: 'User Login',
    type: 'http',
    method: 'POST',
    path: '/api/v1/auth/login',
    x: 80,
    y: 160,
    isInitial: true,
    headers: { 'Content-Type': 'application/json' },
    authBearer: '',
    body: '{"username": "tester", "password": "secret"}',
    assertions: [{ type: 'status_in_range', min_code: 200, max_code: 200 }],
    extractors: [{ source: 'body_json', path: 'token', target_var: 'auth_token' }],
    transitions: [
      { target_step_id: 'get_cart', probability: 0.8 },
      { target_step_id: 'think_time', probability: 0.2 }
    ]
  },
  {
    id: 'think_time',
    name: 'Browse Catalog Delay',
    type: 'delay',
    delayMs: 1500,
    x: 460,
    y: 80,
    transitions: [
      { target_step_id: 'get_cart', probability: 1.0 }
    ]
  },
  {
    id: 'get_cart',
    name: 'Fetch Shopping Cart',
    type: 'http',
    method: 'GET',
    path: '/api/v1/cart',
    x: 840,
    y: 160,
    headers: { 'Accept': 'application/json' },
    authBearer: '{{auth_token}}',
    assertions: [{ type: 'status_in_range', min_code: 200, max_code: 200 }],
    transitions: [
      { target_step_id: 'checkout', probability: 0.7 },
      { target_step_id: 'END', probability: 0.3 }
    ]
  },
  {
    id: 'checkout',
    name: 'Process Checkout',
    type: 'http',
    method: 'POST',
    path: '/api/v1/cart/checkout',
    x: 1220,
    y: 160,
    headers: { 'Content-Type': 'application/json' },
    authBearer: '{{auth_token}}',
    body: '{"payment_method": "qris"}',
    assertions: [{ type: 'status_in_range', min_code: 200, max_code: 200 }],
    transitions: [
      { target_step_id: 'END', probability: 1.0 }
    ]
  }
];

// Maps the editor's internal view mode to/from the app-wide `activeSubTab` string.
// 'presets'/scenario-list is intentionally absent: the list is the outer landing
// page (VisualFlowBuilder's `mode`), not one of the editor's own view tabs.
export const SUBTAB_TO_VIEW_MODE: Record<string, 'visual' | 'code' | 'importer'> = {
  canvas: 'visual',
  generator: 'importer',
  inspect: 'code',
};

export const VIEW_MODE_TO_SUBTAB: Record<string, string> = {
  visual: 'canvas',
  importer: 'generator',
  code: 'inspect',
};
