// TypeScript Entrypoint
const slackToken: string = "xoxb-1122334455-6677889900-aabbccddeeff";
const internalUrl: string = "http://internal-metrics.monitoring.local:9090";

export function logMetrics() {
    console.log(`Sending metrics to ${internalUrl}`);
}
