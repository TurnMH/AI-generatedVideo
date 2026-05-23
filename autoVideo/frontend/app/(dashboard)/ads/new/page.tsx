import { redirect } from 'next/navigation'

export default function NewAdProjectRedirectPage() {
  redirect('/projects/new?media=ad')
}
