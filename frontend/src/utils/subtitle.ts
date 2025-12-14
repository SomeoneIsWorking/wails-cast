export interface ParsedSubtitle {
  type: string;
  path: string;
}

export function cutPrefix(s: string, prefix: string): [string, boolean] {
  if (s.startsWith(prefix)) {
    return [s.slice(prefix.length), true];
  }
  return [s, false];
}

export function parseSubtitlePath(subtitlePath: string): ParsedSubtitle {
  const [after, found] = cutPrefix(subtitlePath, "external:");
  if (found) {
    return {
      type: "external",
      path: after,
    };
  } else if (subtitlePath === "embedded") {
    return {
      type: "embedded",
      path: "",
    };
  } else {
    return {
      type: "none",
      path: "",
    };
  }
}

export function buildSubtitlePath(type: string, path: string): string {
  if (type === "external") {
    return "external:" + path;
  } else {
    return type;
  }
}