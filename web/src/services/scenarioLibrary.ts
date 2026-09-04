import { FlowNode, SavedScenario } from '../types/models';

const STORAGE_KEY = 'oshimai_saved_scenarios';

function readAll(): SavedScenario[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

function writeAll(items: SavedScenario[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(items));
}

// Client-side CRUD for user-authored scenario templates. There is no backend
// endpoint for saved scenarios, so this lives entirely in the browser's
// localStorage — scoped to this device/browser only.
export const scenarioLibrary = {
  list(): SavedScenario[] {
    return readAll().sort((a, b) => b.savedAt.localeCompare(a.savedAt));
  },

  save(name: string, scenarioId: string, baseUrl: string, nodes: FlowNode[]): SavedScenario {
    const entry: SavedScenario = {
      id: `tpl_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 7)}`,
      name,
      savedAt: new Date().toISOString(),
      scenarioId,
      baseUrl,
      nodes,
    };
    const items = readAll();
    items.push(entry);
    writeAll(items);
    return entry;
  },

  rename(id: string, name: string) {
    writeAll(readAll().map(s => (s.id === id ? { ...s, name } : s)));
  },

  // Overwrites an existing template's content in place (used when the editor
  // saves changes back to the scenario it opened, instead of creating a new entry).
  update(id: string, patch: { scenarioId: string; baseUrl: string; nodes: FlowNode[] }) {
    writeAll(readAll().map(s => (s.id === id ? { ...s, ...patch, savedAt: new Date().toISOString() } : s)));
  },

  remove(id: string) {
    writeAll(readAll().filter(s => s.id !== id));
  },
};
