const size = (n) => {
    if (n < 1024) {
        return n.toFixed(0)+"B"
    }
    if (n < 1024 * 1024) {
        return (n/1024).toFixed(0)+"KiB"
    }
    if (n < 1024 * 1024 * 1024) {
        return (n/(1024 * 1024)).toFixed(0)+"MiB"
    }
    if (n < 1024 * 1024 * 1024 * 1024) {
        return (n/(1024 * 1024 * 1024)).toFixed(0)+"GiB"
    }
    return (n/(1024 * 1024 * 1024 * 1024)).toFixed(0)+"TiB"
}

const labelUtils = {
  size
}

export default labelUtils