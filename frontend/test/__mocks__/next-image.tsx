import React from "react"

type Props = {
  src: string
  alt: string
  width?: number
  height?: number
  className?: string
  [key: string]: unknown
}

export default function MockImage({ src, alt, width, height, className }: Props) {
  // eslint-disable-next-line @next/next/no-img-element
  return <img src={src} alt={alt} width={width} height={height} className={className} />
}
