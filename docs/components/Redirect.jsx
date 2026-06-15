import { useEffect } from 'react'
import { useRouter } from 'next/router'

export default function Redirect({ to }) {
  const router = useRouter()

  useEffect(() => {
    router.replace(to)
  }, [to, router])

  return (
    <p>
      This page has moved. <a href={to}>Continue →</a>
    </p>
  )
}
