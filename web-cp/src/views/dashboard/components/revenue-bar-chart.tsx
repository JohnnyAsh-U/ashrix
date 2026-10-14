import { ResponsiveContainer, BarChart, CartesianGrid, XAxis, YAxis, Tooltip, Legend, Bar } from 'recharts'

type Props = { data: any[]; isLoading?: boolean }

export function RevenueBarChart({ data, isLoading }: Props) {
  if (!data || isLoading) return null

  return (
    <div className="bg-white rounded-lg border p-6">
      <h3 className="text-lg font-semibold mb-4">Revenue Trend (Last 12 Months)</h3>
      <ResponsiveContainer width="100%" height={300}>
        <BarChart data={data}>
          <CartesianGrid strokeDasharray="3 3" />
          <XAxis dataKey="month" />
          <YAxis />
          <Tooltip />
          <Legend />
          <Bar dataKey="revenue" fill="#3b82f6" name="Revenue ($)" />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}

export default RevenueBarChart
