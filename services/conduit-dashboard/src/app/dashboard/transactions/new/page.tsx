import { Card } from "@/components/ui/Card";
import { NewPaymentForm } from "./NewPaymentForm";

export default function NewPaymentPage() {
  // Minted once, when this page renders — not inside the server action —
  // so a double-click or a network retry of the same form submission
  // reuses this exact key and replays core's original response instead of
  // creating a second payment intent. A fresh page load (a genuinely new
  // "New Payment" visit) gets a fresh key. See coreClient.createPaymentIntent's
  // doc comment for the full reasoning.
  const idempotencyKey = crypto.randomUUID();

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">New payment</h1>
      <Card className="max-w-md p-6">
        <NewPaymentForm idempotencyKey={idempotencyKey} />
      </Card>
    </div>
  );
}
