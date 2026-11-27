// 悬空镜像（<none>）没有可用的 name:tag 引用，回退到镜像 ID
export function runnableImageRef(img: { id: string; name: string; tag: string }): string {
  if (!img.name || img.name === '<none>' || !img.tag || img.tag === '<none>') {
    return img.id
  }
  return `${img.name}:${img.tag}`
}
