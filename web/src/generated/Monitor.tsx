// Generated from screens/Monitor.aui; edit the AUI source.
import { Badge, Button, Card, Checkbox, Field, Grid, Heading, Input, Link, Metric, Page, Row, Section, Select, Spacer, Text } from "@/components/ui"

export function Monitor({ data, onAction }: { data: any; onAction: (name: string) => void }) {
  return (
    <Page>
      <Row>
        <Heading level={1}>Workspace health</Heading>
        <Spacer />
        <Button onClick={() => onAction("refresh")}>Check now</Button>
        <Button variant="secondary" onClick={() => onAction("newCheck")}>+ Add monitor</Button>
      </Row>
      <Grid columns={3}>
        <Card>
          <Text variant="caption">MEMORY IN USE</Text>
          <Metric value={data.memory} />
        </Card>
        <Card>
          <Text variant="caption">DISK AVAILABLE</Text>
          <Metric value={data.disk} />
        </Card>
        <Card>
          <Text variant="caption">CPU IN USE</Text>
          <Metric value={data.cpu} />
        </Card>
      </Grid>
      <Card>
        <Heading level={2}>Maintenance & recovery</Heading>
        <Text>{data.update}</Text>
        <Text>{data.backup}</Text>
        <Text variant="caption">Sample history: 30 days. Metrics and infrastructure details are visible to the server owner only.</Text>
      </Card>
      {data.checks.map((item, i) => (
        <Card key={i} variant={item.variant}>
          <Row>
            <Heading level={2}>{item.name}</Heading>
            <Badge tone={item.tone}>{item.state}</Badge>
            <Spacer />
            <Text>{item.uptimeLabel}</Text>
            <Button variant="secondary" item={item.id} onClick={() => onAction("edit")}>Settings</Button>
            <Button variant="secondary" item={item.id} onClick={() => onAction("run")}>Run check</Button>
            <Button variant="secondary" item={item.id} onClick={() => onAction("samples")}>History</Button>
          </Row>
          <Text variant="caption">{item.detail}</Text>
        </Card>
      ))}
      {data.empty && (
        <Card variant="empty">
          <Heading level={2}>No monitors yet</Heading>
          <Text>Managed installations discover Workspace apps automatically. You can also monitor public HTTP/TCP endpoints or a Windows service.</Text>
        </Card>
      )}
      {data.editing && (
        <Card>
          <Heading level={2}>Monitor settings</Heading>
          <Field label="Name"><Input name="name" /></Field>
          <Field label="Kind"><Select name="kind" options={data.kinds} /></Field>
          <Field label="Target (URL, host:port, discovered app ID or service name)"><Input name="target" /></Field>
          <Grid columns={3}>
            <Field label="Interval in seconds"><Input name="interval" type="number" min={15} max={86400} /></Field>
            <Field label="Failures before incident"><Input name="threshold" type="number" min={1} max={20} /></Field>
            <Field label="Alert quiet period in seconds"><Input name="quiet" type="number" min={60} max={86400} /></Field>
          </Grid>
          <Field label="Check timeout in seconds (1–60)"><Input name="timeout" type="number" min={1} max={60} /></Field>
          <Field label="Maintenance until (ISO 8601 with offset, optional)"><Input name="maintenance" /></Field>
          <Checkbox name="enabled" label="Enable checks" />
          <Checkbox name="public" label="Include safe name and state on public status page" />
          <Row>
            <Button onClick={() => onAction("save")}>Save monitor</Button>
            <Button variant="secondary" onClick={() => onAction("close")}>Close</Button>
            <Button variant="danger" onClick={() => onAction("delete")}>Delete custom monitor</Button>
          </Row>
        </Card>
      )}
      {data.hasSamples && (
        <Card>
          <Heading level={2}>Recent samples</Heading>
          <Section id="uptime-chart" />
          {data.samples.map((item, i) => (
            <Row key={i}>
              <Text>{item.at}</Text>
              <Badge>{item.state}</Badge>
              <Text>{item.latencyLabel}</Text>
              <Text>{item.detail}</Text>
            </Row>
          ))}
        </Card>
      )}
      <Card>
        <Heading level={2}>Incidents</Heading>
        {data.incidents.map((item, i) => (
          <Row key={i}>
            <Text>{item.name}</Text>
            <Text variant="caption">{item.detail}</Text>
            <Button variant="secondary" item={item.id} onClick={() => onAction("ack")}>{item.ackLabel}</Button>
          </Row>
        ))}
      </Card>
      <Card>
        <Heading level={2}>Alerts & public status</Heading>
        <Checkbox name="publish" label="Publish opt-in service names and states (off by default)" />
        <Link href={data.statusURL}>View public status JSON</Link>
        <Field label="HTTPS webhook (leave blank to preserve existing; CLEAR to remove)"><Input name="webhook" type="password" /></Field>
        <Text variant="caption">Webhook URLs are encrypted at rest and omitted from responses and logs. Acknowledging an incident suppresses further alerts.</Text>
        <Button onClick={() => onAction("settings")}>Save settings</Button>
      </Card>
    </Page>
  )
}
