import { generateStaticIdParam } from '@/lib/static-export'

import CharactersPageClient from './page-client'

export function generateStaticParams() {
  return generateStaticIdParam()
}

export default function CharactersPage() {
  return <CharactersPageClient />
}