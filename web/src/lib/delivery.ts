import type { DeliveryStatus } from '../api/types'

export const DELIVERY_STATUS_LABEL: Record<DeliveryStatus, string> = {
  pending: 'Menunggu',
  on_the_way: 'Dalam perjalanan',
  delivered: 'Terkirim',
  failed: 'Gagal',
  cancelled: 'Dibatalkan',
}

// Pengiriman yang jadwalnya masih relevan untuk peringatan jatuh tempo.
export const isScheduled = (status: DeliveryStatus) => status === 'pending' || status === 'on_the_way'

// Pengiriman yang masih berjalan: ikut berubah bersama pre-order-nya, dan ikut terkirim saat pre-order-nya selesai.
export const isActiveDelivery = (status: DeliveryStatus | null | undefined) =>
  status === 'pending' || status === 'on_the_way' || status === 'failed'
