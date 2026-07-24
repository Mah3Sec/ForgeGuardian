// FORGEGUARDIAN TEST FIXTURE — inert payload with detectable patterns

// Pattern: prototype pollution
function merge(target, source) {
  for (const key in source) {
    // Semgrep rule: prototype-pollution-via-bracket-assignment
    target[key] = source[key]; // should flag: no hasOwnProperty check
  }
  return target;
}

// Pattern: eval usage
function runTemplate(tpl, data) {
  // Semgrep rule: dangerous-eval
  return eval('`' + tpl + '`'); // intentional dangerous pattern
}

// Pattern: SQL injection via string concatenation
function buildQuery(userInput) {
  // Semgrep rule: sql-injection
  return 'SELECT * FROM users WHERE name = \'' + userInput + '\'';
}

module.exports = { merge, runTemplate, buildQuery };
