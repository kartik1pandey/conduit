import { Card } from "@/components/ui/Card";
import { RegisterWebhookForm } from "./RegisterWebhookForm";

export default function NewWebhookEndpointPage() {
  // Same "minted at render, carried as a hidden field" reasoning as the
  // New Payment page — see coreClient.createPaymentIntent's doc comment.
  const idempotencyKey = crypto.randomUUID();

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Register webhook endpoint</h1>
      <Card className="max-w-md p-6">
        <RegisterWebhookForm idempotencyKey={idempotencyKey} />
      </Card>
    </div>
  );
}
