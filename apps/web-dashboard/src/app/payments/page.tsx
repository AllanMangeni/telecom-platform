import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { StatCard } from "@/components/stat-card";
import { CreditCard, ArrowUpRight, ArrowDownRight, Clock } from "lucide-react";

const transactions = [
  { id: "txn_001", subscriber: "Alice Martin", type: "TOP_UP", amount: 50.0, status: "completed", date: "2026-04-14 14:23" },
  { id: "txn_002", subscriber: "Bob Dupont", type: "TOP_UP", amount: 25.0, status: "completed", date: "2026-04-14 13:15" },
  { id: "txn_003", subscriber: "David Leroy", type: "TOP_UP", amount: 10.0, status: "failed", date: "2026-04-14 12:45" },
  { id: "txn_004", subscriber: "Emma Petit", type: "REFUND", amount: -15.0, status: "completed", date: "2026-04-14 11:30" },
  { id: "txn_005", subscriber: "Henri Faure", type: "TOP_UP", amount: 100.0, status: "pending", date: "2026-04-14 10:00" },
  { id: "txn_006", subscriber: "François Blanc", type: "TOP_UP", amount: 30.0, status: "completed", date: "2026-04-13 22:15" },
];

export default function PaymentsPage() {
  return (
    <div className="p-8 space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Payments</h1>
        <p className="text-muted-foreground mt-1">Transaction history and payment management.</p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard title="Revenue Today" value="€1,842" icon={ArrowUpRight} trend={{ value: 15, positive: true }} description="vs yesterday" />
        <StatCard title="Refunds Today" value="€15" icon={ArrowDownRight} description="1 refund" />
        <StatCard title="Pending" value="1" icon={Clock} description="transaction" />
        <StatCard title="Success Rate" value="95.2%" icon={CreditCard} description="last 7 days" />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Recent Transactions</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b text-left text-muted-foreground">
                  <th className="pb-3 font-medium">Transaction ID</th>
                  <th className="pb-3 font-medium">Subscriber</th>
                  <th className="pb-3 font-medium">Type</th>
                  <th className="pb-3 font-medium text-right">Amount</th>
                  <th className="pb-3 font-medium">Status</th>
                  <th className="pb-3 font-medium text-right">Date</th>
                </tr>
              </thead>
              <tbody>
                {transactions.map((txn) => (
                  <tr key={txn.id} className="border-b last:border-0 hover:bg-muted/50 transition-colors">
                    <td className="py-3 font-mono text-xs">{txn.id}</td>
                    <td className="py-3">{txn.subscriber}</td>
                    <td className="py-3">
                      <Badge variant={txn.type === "REFUND" ? "warning" : "secondary"}>{txn.type}</Badge>
                    </td>
                    <td className="py-3 text-right font-mono">
                      <span className={txn.amount < 0 ? "text-red-600" : ""}>€{Math.abs(txn.amount).toFixed(2)}</span>
                    </td>
                    <td className="py-3">
                      <Badge variant={txn.status === "completed" ? "success" : txn.status === "failed" ? "destructive" : "secondary"}>
                        {txn.status}
                      </Badge>
                    </td>
                    <td className="py-3 text-right text-muted-foreground text-xs">{txn.date}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
