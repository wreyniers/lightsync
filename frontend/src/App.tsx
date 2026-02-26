import { useState } from "react";
import { Layout } from "@/components/Layout";
import { Lights } from "@/components/Lights";
import { Scenes } from "@/components/Scenes";
import { Settings } from "@/components/Settings";

const PAGES = {
  lights: Lights,
  scenes: Scenes,
  settings: Settings,
} as const;

function App() {
  const [currentPage, setCurrentPage] = useState<keyof typeof PAGES>("lights");
  const Page = PAGES[currentPage] ?? Lights;

  return (
    <Layout currentPage={currentPage} onNavigate={(p) => setCurrentPage(p as keyof typeof PAGES)}>
      <Page />
    </Layout>
  );
}

export default App;
