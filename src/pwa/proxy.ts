import { NextResponse, type NextRequest } from "next/server";

// Same-origin /api/v1 requests are forwarded to the shared Go API. The target
// is read at request time from the server-only API_INTERNAL_URL, so one build
// can run against different API hosts. Go remains the authority for all
// authorization and business rules.
export function proxy(request: NextRequest) {
  const base = process.env.API_INTERNAL_URL;
  if (!base) {
    return NextResponse.json(
      {
        error: {
          code: "DEPENDENCY_UNAVAILABLE",
          message: "Service is not configured",
          request_id: "",
          fields: {},
        },
      },
      { status: 503, headers: { "Cache-Control": "no-store" } },
    );
  }
  const target = new URL(request.nextUrl.pathname + request.nextUrl.search, base);
  return NextResponse.rewrite(target);
}

export const config = {
  matcher: "/api/v1/:path*",
};
