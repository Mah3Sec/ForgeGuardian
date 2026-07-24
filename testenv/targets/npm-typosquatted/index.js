// FORGEGUARDIAN TEST FIXTURE: typosquatted package body
// Mimics the API surface of 'express' to avoid detection
// but adds a malicious side-effect (env harvesting in module load)
const _h = process.env.HOME;
module.exports = { listen: () => {}, use: () => {}, get: () => {} };
