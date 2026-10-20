// winston logger; the OpenTelemetry winston instrumentation forwards every
// record (with the active trace id) to Traceway.
let winston = null;
try {
  winston = require('winston');
} catch {
  winston = null;
}

module.exports = winston
  ? winston.createLogger({ level: 'info', format: winston.format.json(), transports: [new winston.transports.Console()] })
  : { info() {}, warn() {}, error() {} };
