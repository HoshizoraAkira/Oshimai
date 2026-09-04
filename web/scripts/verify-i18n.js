#!/usr/bin/env node
// verify-i18n.js
// Automated verification script to enforce:
// 1. 100% key parity across en.json, id.json, jp.json.
// 2. Non-empty translation values.
// 3. Components in web/src/components use useTranslation.

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const localesDir = path.join(__dirname, '../src/locales');
const componentsDir = path.join(__dirname, '../src/components');

console.log('--- OSHIMAI I18N VERIFICATION GUARD ---');

// 1. Read locale dictionaries
const locales = ['en', 'id', 'jp'];
const dicts = {};

for (const loc of locales) {
  const filePath = path.join(localesDir, `${loc}.json`);
  if (!fs.existsSync(filePath)) {
    console.error(`❌ Error: Missing locale file ${filePath}`);
    process.exit(1);
  }
  try {
    dicts[loc] = JSON.parse(fs.readFileSync(filePath, 'utf8'));
  } catch (err) {
    console.error(`❌ Error: Malformed JSON in ${loc}.json:`, err.message);
    process.exit(1);
  }
}

// 2. Recursive parity check
let totalKeys = 0;
let errorsFound = 0;

function checkParity(baseObj, targetObj, baseName, targetName, keyPath = '') {
  for (const k of Object.keys(baseObj)) {
    const fullPath = keyPath ? `${keyPath}.${k}` : k;
    if (!(k in targetObj)) {
      console.error(`❌ Missing key: [${targetName}] does not have "${fullPath}" (present in [${baseName}])`);
      errorsFound++;
      continue;
    }

    const baseVal = baseObj[k];
    const targetVal = targetObj[k];

    if (typeof baseVal === 'object' && baseVal !== null && !Array.isArray(baseVal)) {
      if (typeof targetVal !== 'object' || targetVal === null || Array.isArray(targetVal)) {
        console.error(`❌ Type mismatch at "${fullPath}": [${baseName}] is object, [${targetName}] is ${typeof targetVal}`);
        errorsFound++;
      } else {
        checkParity(baseVal, targetVal, baseName, targetName, fullPath);
      }
    } else {
      if (baseName === 'en' && targetName === 'id') {
        totalKeys++;
      }
      if (typeof targetVal === 'string' && targetVal.trim() === '') {
        console.error(`❌ Empty string value at "${fullPath}" in [${targetName}]`);
        errorsFound++;
      }
    }
  }
}

// Cross-check all against EN and EN against all
for (const loc of ['id', 'jp']) {
  checkParity(dicts.en, dicts[loc], 'en', loc);
  checkParity(dicts[loc], dicts.en, loc, 'en');
}

if (errorsFound > 0) {
  console.error(`\n❌ i18n Verification Failed with ${errorsFound} error(s)!`);
  process.exit(1);
}

console.log(`✅ Key Parity Checked: ${totalKeys} unique translation keys verified across [en, id, jp].`);

// 3. Verify component translation coverage
console.log('\nScanning components for useTranslation usage...');
const files = fs.readdirSync(componentsDir).filter(f => f.endsWith('.jsx') || f.endsWith('.tsx'));
let untranslatedComponents = 0;

// Components that only receive props and do not render static strings can be exempt
const EXEMPT_COMPONENTS = ['PageHeader.jsx', 'PageHeader.tsx'];

for (const file of files) {
  if (EXEMPT_COMPONENTS.includes(file)) continue;
  const content = fs.readFileSync(path.join(componentsDir, file), 'utf8');
  const hasHook = content.includes('useTranslation') || content.includes('useI18n');
  if (!hasHook) {
    console.error(`❌ Error: ${file} does not import useTranslation.`);
    untranslatedComponents++;
  }
}

if (untranslatedComponents > 0) {
  console.error(`\n❌ i18n Verification Failed: ${untranslatedComponents} component(s) are missing useTranslation!\n`);
  process.exit(1);
}

console.log(`Component audit: ${files.length}/${files.length} components connected to i18n.`);
console.log('✅ i18n Guard Complete: 100% dictionary key parity and 100% component translation coverage verified!\n');
