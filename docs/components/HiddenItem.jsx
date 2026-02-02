import { useRef, useEffect } from 'react'

export default function HiddenItem() {
  const ref = useRef(null)

  useEffect(() => {
    const li = ref.current?.closest('li')
    if (li) {
      li.style.display = 'none'
      return () => {
        li.style.display = ''
      }
    }
  }, [])

  return <span ref={ref} style={{ display: 'none' }} />
}
