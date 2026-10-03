import { lazy, Suspense } from "react";
import { Route, Routes } from "react-router-dom";
import PilotApp from "./pilot/PilotApp.js";

const PreviewPage =
  import.meta.env.DEV || import.meta.env.VITE_ENABLE_PREVIEW === "true"
    ? lazy(() => import("./preview/PreviewPage.js"))
    : null;

export default function App() {
  return (
    <Routes>
      {PreviewPage && (
        <Route
          path="/preview"
          element={
            <Suspense fallback={<p className="p-8">Vorschau wird geladen …</p>}>
              <PreviewPage />
            </Suspense>
          }
        />
      )}
      <Route path="*" element={<PilotApp />} />
    </Routes>
  );
}
