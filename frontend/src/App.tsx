import { useState } from "react";
import { Layout } from "@/components/Layout";
import { Lights } from "@/components/Lights";
import { Scenes } from "@/components/Scenes";
import { Settings } from "@/components/Settings";
import { LightsPopupPage } from "@/components/LightsPopupPage";
import { CloseConfirmDialog } from "@/components/CloseConfirmDialog";

const PAGES = {
  lights: Lights,
  scenes: Scenes,
  settings: Settings,
} as const;

function MainApp() {
  const [currentPage, setCurrentPage] = useState<keyof typeof PAGES>("lights");
  const Page = PAGES[currentPage] ?? Lights;

  return (
    <>
      <Layout currentPage={currentPage} onNavigate={(p) => setCurrentPage(p as keyof typeof PAGES)}>
        <Page />
      </Layout>
      <CloseConfirmDialog />
    </>
  );
}

function App() {
  if (window.location.hash === "#lights-popup") {
    return <LightsPopupPage />;
  }
  return <MainApp />;
}

export default App;
