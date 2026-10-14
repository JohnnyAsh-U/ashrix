import { ResponsiveContainer, LineChart, Line, CartesianGrid, XAxis, YAxis, Tooltip, Legend } from 'recharts'

type Props = { data: any[]; isLoading?: boolean }

export function SignupsLineChart({ data, isLoading }: Props) {
  if (!data || isLoading) return null

  return (
    <div className="bg-white rounded-lg border p-6">
      <h3 className="text-lg font-semibold mb-4">New Signups (Last 12 Months)</h3>
      <ResponsiveContainer width="100%" height={300}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="month" />
          <YAxis />
          <Tooltip />
          <Legend />
          <Line
            type="monotone"
            dataKey="signups"
            stroke="#10b981"
            strokeWidth={2}
            name="New Signups"
            dot={{ r: 4 }}
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}

export default SignupsLineChart
