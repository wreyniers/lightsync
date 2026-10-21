import { useState } from "react";
import { Layout } from "@/components/Layout";
import { Lights } from "@/components/Lights";
import { Scenes } from "@/components/Scenes";
import { Settings } from "@/components/Settings";
import { LightsPopupPage } from "@/components/LightsPopupPage";
import { SelfTestPage } from "@/components/selftest/SelfTestPage";
import { SelfTestFixturePage } from "@/components/selftest/SelfTestFixturePage";
import { CloseConfirmDialog } from "@/components/CloseConfirmDialog";

const PAGES = {
  lights: Lights,
  scenes: Scenes,
  settings: Settings,
  selftest: SelfTestPage,
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
  const hash = window.location.hash;
  if (hash === "#lights-popup") {
    return <LightsPopupPage />;
  }
  if (hash === "#self-test-fixture") {
    return <SelfTestFixturePage />;
  }
  return <MainApp />;
}

export default App;
