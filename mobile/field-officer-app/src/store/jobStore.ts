import { create } from 'zustand'
import type { FieldJob, MeterPhoto, OCRResult } from '../types'

interface ActiveJobState {
  activeJob: FieldJob | null
  photos: MeterPhoto[]
  ocrResult: OCRResult | null
  officerNotes: string
  setActiveJob: (job: FieldJob | null) => void
  addPhoto: (photo: MeterPhoto) => void
  setOCRResult: (result: OCRResult) => void
  setOfficerNotes: (notes: string) => void
  clearJob: () => void
}

export const useJobStore = create<ActiveJobState>((set) => ({
  activeJob: null,
  photos: [],
  ocrResult: null,
  officerNotes: '',

  setActiveJob: (job) => set({ activeJob: job, photos: [], ocrResult: null, officerNotes: '' }),
  addPhoto: (photo) => set((state) => ({ photos: [...state.photos, photo] })),
  setOCRResult: (result) => set({ ocrResult: result }),
  setOfficerNotes: (notes) => set({ officerNotes: notes }),
  clearJob: () => set({ activeJob: null, photos: [], ocrResult: null, officerNotes: '' }),
}))
