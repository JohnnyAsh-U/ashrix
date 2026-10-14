
type Props = {
  className? : string
}

export default function Logo({className = "h-24 w-auto"}: Props) {
  return (
   <img src="/icon.png" alt="Ecclesix Core Logo" className={className} />
  )
}