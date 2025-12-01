export function isAcceptedFile(
  filePath: string | undefined,
  acceptedExtensions: string[]
) {
  if (!acceptedExtensions || acceptedExtensions.length === 0) return true;
  const fileLower = (filePath || "").toLowerCase();
  if (!fileLower.includes(".")) return false;
  const ext = fileLower.split(".").pop() || "";
  return acceptedExtensions.some((accepted) => accepted.toLowerCase() === ext);
}

export function isAcceptedFileWithHttp(
  filePath: string,
  acceptedExtensions: string[]
) {
  const lowerPath = filePath.toLowerCase();
  if (lowerPath.startsWith("http://") || lowerPath.startsWith("https://")) {
    return true; // Accept all URLs
  }
  return isAcceptedFile(filePath, acceptedExtensions);
}
