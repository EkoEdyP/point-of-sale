import type { PaymentMethod } from '../api/types'

export const PAYMENT_METHODS: { value: PaymentMethod; label: string }[] = [
  { value: 'cash', label: 'Tunai' },
  { value: 'qris', label: 'QRIS' },
  { value: 'ewallet', label: 'E-wallet' },
  { value: 'card', label: 'Kartu' },
]

export const PAYMENT_LABEL: Record<PaymentMethod, string> = {
  cash: 'Tunai',
  qris: 'QRIS',
  ewallet: 'E-wallet',
  card: 'Kartu',
}

// Nominal uang yang biasa diberikan pelanggan: total dibulatkan ke atas ke kelipatan pecahan umum.
export function cashSuggestions(total: number): number[] {
  const amounts = new Set<number>()
  for (const denomination of [5000, 10000, 20000, 50000, 100000]) {
    const amount = Math.ceil(total / denomination) * denomination
    if (amount > total) amounts.add(amount)
  }
  return [...amounts].sort((a, b) => a - b).slice(0, 4)
}
